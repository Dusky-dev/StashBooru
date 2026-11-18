import React, { useEffect, useMemo, useState } from "react";
import { Button, Col, Form, Row } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { faExchangeAlt, faSignInAlt } from "@fortawesome/free-solid-svg-icons";
import { uniq } from "lodash-es";
import * as GQL from "src/core/generated-graphql";
import {
  getClient,
  queryFindStudio,
  useCopyrightUpdate,
  useStudioUpdate,
} from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import * as FormUtils from "src/utils/form";
import ImageUtils from "src/utils/image";
import { Icon } from "./Icon";
import { ModalComponent } from "./Modal";
import { LoadingIndicator } from "./LoadingIndicator";
import { ScrapeDialog } from "./ScrapeDialog/ScrapeDialog";
import {
  ScrapedImageRow,
  ScrapedInputGroupRow,
  ScrapedStringListRow,
  ScrapedTextAreaRow,
} from "./ScrapeDialog/ScrapeDialogRow";
import { ScrapeResult } from "./ScrapeDialog/scrapeResult";
import {
  Copyright,
  CopyrightSelect,
} from "src/components/Copyrights/CopyrightSelect";
import { Studio, StudioSelect } from "src/components/Studios/StudioSelect";

type Kind = "artist" | "copyright";
type Item = { id: string; name: string };

/** The fields the merge dialog compares for Artists and Copyrights. */
interface Profile {
  id: string;
  name: string;
  aliases: string[];
  description: string;
  image_path?: string | null;
}

interface ChosenValues {
  name?: string;
  aliases?: string[];
  description?: string;
  image?: string;
}

async function loadProfile(kind: Kind, id: string): Promise<Profile> {
  if (kind === "artist") {
    const { data } = await queryFindStudio(id);
    const studio = data.findStudio;
    if (!studio) throw new Error(`Artist ${id} was not found`);
    return {
      id: studio.id,
      name: studio.name,
      aliases: studio.aliases ?? [],
      description: studio.details ?? "",
      image_path: studio.image_path,
    };
  }
  const { data } = await getClient().query<GQL.FindCopyrightQuery>({
    query: GQL.FindCopyrightDocument,
    variables: { id },
    fetchPolicy: "network-only",
  });
  const copyright = data.findCopyright;
  if (!copyright) throw new Error(`Copyright ${id} was not found`);
  return {
    id: copyright.id,
    name: copyright.name,
    aliases: copyright.aliases,
    description: copyright.description,
    image_path: copyright.image_path,
  };
}

/** Field-by-field choice, as in the Character and Tag merge dialogs. */
const ProfileMergeDetails: React.FC<{
  kind: Kind;
  sources: Profile[];
  dest: Profile;
  onClose: (values?: ChosenValues) => void;
}> = ({ kind, sources, dest, onClose }) => {
  const intl = useIntl();
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState(new ScrapeResult<string>(dest.name));
  const [aliases, setAliases] = useState(
    new ScrapeResult<string[]>(dest.aliases)
  );
  const [description, setDescription] = useState(
    new ScrapeResult<string>(dest.description)
  );
  const [image, setImage] = useState(
    new ScrapeResult<string>(dest.image_path ?? undefined)
  );

  useEffect(() => {
    setName(
      new ScrapeResult(dest.name, sources.find((s) => s.name)?.name, false)
    );
    // Source names stay as aliases so imports keep finding the destination.
    const allAliases = uniq(
      dest.aliases.concat(
        sources.map((s) => s.name),
        sources.flatMap((s) => s.aliases)
      )
    );
    setAliases(new ScrapeResult(dest.aliases, allAliases, !!allAliases.length));
    setDescription(
      new ScrapeResult(
        dest.description,
        sources.find((s) => s.description)?.description,
        !dest.description
      )
    );
    async function loadImages() {
      const src = sources.find((s) => s.image_path);
      if (src?.image_path && dest.image_path) {
        const [destData, srcData] = await Promise.all([
          ImageUtils.imageToDataURL(dest.image_path),
          ImageUtils.imageToDataURL(src.image_path),
        ]);
        setImage(new ScrapeResult(destData, srcData, false));
      } else {
        setImage(
          new ScrapeResult(
            dest.image_path ?? undefined,
            src?.image_path ?? undefined,
            !dest.image_path
          )
        );
      }
      setLoading(false);
    }
    void loadImages().catch(() => setLoading(false));
  }, [sources, dest]);

  function createValues(): ChosenValues {
    const chosenName = name.getNewValue() ?? dest.name;
    return {
      name: chosenName,
      aliases: aliases
        .getNewValue()
        ?.map((s) => s.trim())
        .filter(
          (s) => s.length > 0 && s.toLowerCase() !== chosenName.toLowerCase()
        ),
      description: description.getNewValue(),
      image: image.useNewValue ? image.getNewValue() : undefined,
    };
  }

  return (
    <ScrapeDialog
      className="profile-merge-dialog"
      title={intl.formatMessage({ id: "actions.merge" })}
      existingLabel={intl.formatMessage({ id: "dialogs.merge.destination" })}
      scrapedLabel={intl.formatMessage({ id: "dialogs.merge.combined" })}
      onClose={(apply) => onClose(apply ? createValues() : undefined)}
    >
      {loading ? (
        <LoadingIndicator />
      ) : (
        <>
          <ScrapedInputGroupRow
            field="name"
            title={intl.formatMessage({ id: "name" })}
            result={name}
            onChange={setName}
          />
          <ScrapedStringListRow
            field="aliases"
            title={intl.formatMessage({ id: "aliases" })}
            result={aliases}
            onChange={setAliases}
          />
          <ScrapedTextAreaRow
            field="description"
            title={intl.formatMessage({
              id: kind === "artist" ? "details" : "description",
            })}
            result={description}
            onChange={setDescription}
          />
          <ScrapedImageRow
            field="image"
            title={intl.formatMessage({ id: "image" })}
            className="performer-image"
            result={image}
            onChange={setImage}
          />
        </>
      )}
    </ScrapeDialog>
  );
};

/**
 * Artist and Copyright merges work like Character and Tag merges: several
 * sources, a swappable destination, then a choice per field. Linked media,
 * identities and aliases move to the destination in one transaction.
 */
export const ProfileMergeModal: React.FC<{
  kind: Kind;
  current: Item;
  show: boolean;
  onClose: (mergedID?: string) => void;
}> = ({ kind, current, show, onClose }) => {
  const intl = useIntl();
  const Toast = useToast();
  const [src, setSrc] = useState<Item[]>([]);
  const [dest, setDest] = useState<Item | null>(current);
  const [loaded, setLoaded] = useState<{ sources: Profile[]; dest: Profile }>();
  const [running, setRunning] = useState(false);
  // Set once the sources are merged: retrying then only saves the values.
  const [merged, setMerged] = useState(false);
  const [updateStudio] = useStudioUpdate();
  const [updateCopyright] = useCopyrightUpdate();

  // Reset only when the dialog opens or the profile changes: the page passes
  // a new `current` object on every render (a toast re-renders it too).
  const currentID = current.id;
  const currentName = current.name;
  useEffect(() => {
    if (show) {
      setSrc([]);
      setDest({ id: currentID, name: currentName });
      setLoaded(undefined);
      setMerged(false);
    }
  }, [show, currentID, currentName]);

  const srcIDs = useMemo(() => src.map((s) => s.id), [src]);
  const destIDs = useMemo(() => (dest ? [dest.id] : []), [dest]);

  async function loadProfiles() {
    if (!dest) return;
    try {
      setRunning(true);
      const [destination, ...sources] = await Promise.all(
        [dest.id, ...srcIDs].map((id) => loadProfile(kind, id))
      );
      setLoaded({ sources, dest: destination });
    } catch (e) {
      Toast.error(e);
    } finally {
      setRunning(false);
    }
  }

  async function merge(values: ChosenValues) {
    if (!dest) return;
    setRunning(true);
    if (!merged)
      try {
        const response = await fetch(
          kind === "artist" ? "/studio/merge" : "/tag/copyright/merge",
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              sourceIDs: srcIDs.map(Number),
              destinationID: Number(dest.id),
              keepDestination: true,
            }),
          }
        );
        if (!response.ok) throw new Error(await response.text());
        setMerged(true);
      } catch (e) {
        // Nothing changed: the dialog stays open with the same choices.
        Toast.error(e);
        setRunning(false);
        return;
      }
    // The sources are gone; apply the chosen values to the destination now
    // that a source's name can be reused.
    try {
      if (kind === "artist") {
        await updateStudio({
          variables: {
            input: {
              id: dest.id,
              name: values.name,
              aliases: values.aliases,
              details: values.description,
              image: values.image,
            },
          },
        });
      } else {
        await updateCopyright({
          variables: {
            input: {
              id: dest.id,
              name: values.name,
              aliases: values.aliases,
              description: values.description,
              image: values.image,
            },
          },
        });
      }
      Toast.success(
        intl.formatMessage({
          id:
            kind === "artist"
              ? "toast.merged_artists"
              : "toast.merged_copyrights",
        })
      );
      setLoaded(undefined);
      onClose(dest.id);
    } catch (e) {
      // The merge is done; keep the choices open so a rejected value (for
      // example a name another record already uses) can be fixed and saved.
      Toast.error(
        intl.formatMessage(
          { id: "dialogs.merge.values_not_saved" },
          { error: e instanceof Error ? e.message : String(e) }
        )
      );
    } finally {
      setRunning(false);
    }
  }

  if (loaded) {
    return (
      <ProfileMergeDetails
        kind={kind}
        sources={loaded.sources}
        dest={loaded.dest}
        onClose={(values) => {
          if (values) {
            void merge(values);
            return;
          }
          setLoaded(undefined);
          onClose(merged ? dest?.id : undefined);
        }}
      />
    );
  }

  const select = (
    values: Item[],
    onSelect: (items: Item[]) => void,
    multi: boolean,
    exclude: string[]
  ) =>
    kind === "artist" ? (
      <StudioSelect
        isMulti={multi}
        creatable={false}
        values={values as Studio[]}
        onSelect={onSelect}
        excludeIds={exclude}
        menuPortalTarget={document.body}
      />
    ) : (
      <CopyrightSelect
        isMulti={multi}
        creatable={false}
        values={values as Copyright[]}
        onSelect={onSelect}
        excludeIds={exclude}
        menuPortalTarget={document.body}
      />
    );

  const label = (id: string) =>
    FormUtils.renderLabel({
      title: intl.formatMessage({ id }),
      labelProps: { column: true, sm: 3, xl: 12 },
    });

  return (
    <ModalComponent
      show={show}
      header={intl.formatMessage({ id: "actions.merge" })}
      icon={faSignInAlt}
      accept={{
        text: intl.formatMessage({ id: "actions.merge" }),
        onClick: () => void loadProfiles(),
      }}
      disabled={!src.length || !dest}
      cancel={{ variant: "secondary", onClick: () => onClose() }}
      isRunning={running}
    >
      <div className="form-container row px-3">
        <div className="col-12 col-lg-6 col-xl-12">
          <Form.Group controlId="profile-merge-source" as={Row}>
            {label("dialogs.merge.source")}
            <Col sm={9} xl={12}>
              {select(src, setSrc, true, destIDs)}
            </Col>
          </Form.Group>
          <Form.Group as={Row} className="justify-content-center">
            <Button
              variant="secondary"
              onClick={() => {
                if (!src.length || !dest) return;
                setDest(src[0]);
                setSrc([...src.slice(1), dest]);
              }}
              disabled={!src.length || !dest}
              title={intl.formatMessage({ id: "actions.swap" })}
            >
              <Icon className="fa-fw" icon={faExchangeAlt} />
            </Button>
          </Form.Group>
          <Form.Group controlId="profile-merge-destination" as={Row}>
            {label("dialogs.merge.destination")}
            <Col sm={9} xl={12}>
              {select(
                dest ? [dest] : [],
                (items) => setDest(items[0] ?? null),
                false,
                srcIDs
              )}
            </Col>
          </Form.Group>
        </div>
      </div>
    </ModalComponent>
  );
};

/** The detail page's Merge button; the current profile is the destination. */
export const ProfileMergeButton: React.FC<{
  kind: Kind;
  current: Item;
  onMerged: (destinationID: string) => void;
}> = ({ kind, current, onMerged }) => {
  const [show, setShow] = useState(false);
  return (
    <>
      <Button variant="secondary" onClick={() => setShow(true)}>
        <FormattedMessage id="actions.merge" />
        ...
      </Button>
      <ProfileMergeModal
        kind={kind}
        current={current}
        show={show}
        onClose={(mergedID) => {
          setShow(false);
          if (mergedID !== undefined && mergedID !== current.id)
            onMerged(mergedID);
        }}
      />
    </>
  );
};
