import React, { useEffect, useState } from "react";
import { Accordion, Button, Card } from "react-bootstrap";
import { FormattedMessage, FormattedTime, useIntl } from "react-intl";
import { TruncatedText } from "src/components/Shared/TruncatedText";
import { DeleteFilesDialog } from "src/components/Shared/DeleteFilesDialog";
import { RevealInFilesystemButton } from "src/components/Shared/RevealInFilesystemButton";
import * as GQL from "src/core/generated-graphql";
import { mutateImageSetPrimaryFile } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import TextUtils from "src/utils/text";
import { TextField, URLField, URLsField } from "src/utils/field";
import { FileSize } from "src/components/Shared/FileSize";
import NavUtils from "src/utils/navigation";
import { useConfigurationContext } from "src/hooks/Config";
import { DetailsItem } from "./ImageDetailsCustomize";

interface IFileInfoPanelProps {
  file: GQL.ImageFileDataFragment | GQL.VideoFileDataFragment;
  image?: GQL.ImageDataFragment;
  primary?: boolean;
  ofMany?: boolean;
  onSetPrimaryFile?: () => void;
  onDeleteFile?: () => void;
  loading?: boolean;
}

interface FilenameToken {
  token: string;
  value: string;
  values?: string[];
}

const tokenLabels: Record<string, string> = {
  artist: "image_details.filename_tokens.artist",
  copyright: "image_details.filename_tokens.copyright",
  character: "image_details.filename_tokens.character",
  md5: "image_details.filename_tokens.md5",
  ext: "image_details.filename_tokens.ext",
};

// Names as the filename tagging source reads them: underscores are spaces.
function displayName(token: string, value: string) {
  if (token === "ext") return value.toUpperCase();
  if (token === "md5") return value;
  return value.replace(/_/g, " ");
}

// What the tagging filename layout reads from the primary file's name, by
// the same parser as the filename tagging source: one chip per name. Nothing
// when the name does not match the layout.
const FilenameTokens: React.FC<{ imageId: string; path: string }> = ({
  imageId,
  path,
}) => {
  const intl = useIntl();
  const [tokens, setTokens] = useState<FilenameToken[]>([]);
  // biome-ignore lint/correctness/useExhaustiveDependencies: a renamed or moved file is read again.
  useEffect(() => {
    const controller = new AbortController();
    setTokens([]);
    void fetch(`image/${imageId}/filename-tokens`, {
      signal: controller.signal,
    })
      .then((response) =>
        response.ok ? response.json() : { tokens: [] as FilenameToken[] }
      )
      .then((result: { tokens?: FilenameToken[] }) => {
        if (!controller.signal.aborted) setTokens(result.tokens ?? []);
      })
      .catch(() => {});
    return () => controller.abort();
  }, [imageId, path]);
  if (tokens.length === 0) return null;
  return (
    <>
      <dt>
        <FormattedMessage id="image_details.filename_tokens.heading" />:
      </dt>
      <dd className="filename-tokens">
        {tokens.map(({ token, value, values }) => (
          <div className="filename-token" key={token}>
            <span className="filename-token-name">
              {tokenLabels[token]
                ? intl.formatMessage({ id: tokenLabels[token] })
                : token}
            </span>
            <span className="filename-token-values">
              {(values?.length ? values : [value]).map((item) => (
                <span
                  className={`filename-token-value filename-token-${token}`}
                  key={item}
                  title={item}
                >
                  {displayName(token, item)}
                </span>
              ))}
            </span>
          </div>
        ))}
      </dd>
    </>
  );
};

const FileInfoPanel: React.FC<IFileInfoPanelProps> = (
  props: IFileInfoPanelProps
) => {
  const intl = useIntl();
  const { configuration } = useConfigurationContext();
  const showTokens = configuration?.ui?.showImageFilenameTokens ?? false;
  const checksum = props.file.fingerprints.find((f) => f.type === "md5");
  const phash = props.file.fingerprints.find((f) => f.type === "phash");

  return (
    <div>
      <dl className="container image-file-info details-list">
        {props.primary && (
          <>
            <dt></dt>
            <dd className="primary-file">
              <FormattedMessage id="primary_file" />
            </dd>
          </>
        )}
        <DetailsItem row id="path" label={intl.formatMessage({ id: "path" })}>
          <TextField id="path">
            <span className="d-flex align-items-center">
              <span className="image-file-path">{props.file.path}</span>
              <RevealInFilesystemButton fileId={props.file.id} />
            </span>
          </TextField>
        </DetailsItem>
        <DetailsItem
          row
          id="media_info.md5"
          label={intl.formatMessage({ id: "media_info.md5" })}
        >
          <TextField id="media_info.md5" value={checksum?.value} />
        </DetailsItem>
        <DetailsItem
          row
          id="media_info.phash"
          label={intl.formatMessage({ id: "media_info.phash" })}
        >
          <URLField
            id="media_info.phash"
            abbr={intl.formatMessage({ id: "media_info.phash_meaning" })}
            value={phash?.value}
            url={NavUtils.makeImagesPHashMatchUrl(phash?.value)}
            target="_self"
            internal
          />
        </DetailsItem>
        {showTokens && props.image && (props.primary || !props.ofMany) && (
          <DetailsItem
            row
            id="filename_tokens"
            label={intl.formatMessage({
              id: "image_details.filename_tokens.heading",
            })}
          >
            <FilenameTokens imageId={props.image.id} path={props.file.path} />
          </DetailsItem>
        )}
        <DetailsItem
          row
          id="filesize"
          label={intl.formatMessage({ id: "filesize" })}
        >
          <TextField id="filesize">
            <span className="text-truncate">
              <FileSize size={props.file.size} />
            </span>
          </TextField>
        </DetailsItem>
        {props.image ? (
          <>
            <DetailsItem
              row
              id="urls"
              label={intl.formatMessage({ id: "urls" })}
            >
              <URLsField id="urls" urls={props.image.urls} />
            </DetailsItem>
            <DetailsItem
              row
              id="created_at"
              label={intl.formatMessage({ id: "created_at" })}
            >
              <TextField
                id="created_at"
                value={TextUtils.formatDateTime(intl, props.image.created_at)}
              />
            </DetailsItem>
            <DetailsItem
              row
              id="updated_at"
              label={intl.formatMessage({ id: "updated_at" })}
            >
              <TextField
                id="updated_at"
                value={TextUtils.formatDateTime(intl, props.image.updated_at)}
              />
            </DetailsItem>
          </>
        ) : null}
        <DetailsItem
          row
          id="file_mod_time"
          label={intl.formatMessage({ id: "file_mod_time" })}
        >
          <TextField id="file_mod_time">
            <FormattedTime
              dateStyle="medium"
              timeStyle="medium"
              value={props.file.mod_time ?? 0}
            />
          </TextField>
        </DetailsItem>
        <DetailsItem
          row
          id="dimensions"
          label={intl.formatMessage({ id: "dimensions" })}
        >
          <TextField
            id="dimensions"
            value={`${props.file.width} x ${props.file.height}`}
            truncate
          />
        </DetailsItem>
      </dl>
      {props.ofMany && props.onSetPrimaryFile && !props.primary && (
        <div>
          <Button
            className="edit-button"
            disabled={props.loading}
            onClick={props.onSetPrimaryFile}
          >
            <FormattedMessage id="actions.make_primary" />
          </Button>
          <Button
            variant="danger"
            disabled={props.loading}
            onClick={props.onDeleteFile}
          >
            <FormattedMessage id="actions.delete_file" />
          </Button>
        </div>
      )}
    </div>
  );
};
interface IImageFileInfoPanelProps {
  image: GQL.ImageDataFragment;
}

export const ImageFileInfoPanel: React.FC<IImageFileInfoPanelProps> = (
  props: IImageFileInfoPanelProps
) => {
  const Toast = useToast();
  const intl = useIntl();

  const [loading, setLoading] = useState(false);
  const [deletingFile, setDeletingFile] = useState<
    GQL.ImageFileDataFragment | GQL.VideoFileDataFragment | undefined
  >();

  if (props.image.visual_files.length === 0) {
    return (
      <dl className="container image-file-info details-list">
        <DetailsItem row id="urls" label={intl.formatMessage({ id: "urls" })}>
          <URLsField id="urls" urls={props.image.urls} />
        </DetailsItem>
        <DetailsItem
          row
          id="created_at"
          label={intl.formatMessage({ id: "created_at" })}
        >
          <TextField
            id="created_at"
            value={TextUtils.formatDateTime(intl, props.image.created_at)}
          />
        </DetailsItem>
        <DetailsItem
          row
          id="updated_at"
          label={intl.formatMessage({ id: "updated_at" })}
        >
          <TextField
            id="updated_at"
            value={TextUtils.formatDateTime(intl, props.image.updated_at)}
          />
        </DetailsItem>
      </dl>
    );
  }

  if (props.image.visual_files.length === 1) {
    return (
      <FileInfoPanel file={props.image.visual_files[0]} image={props.image} />
    );
  }

  async function onSetPrimaryFile(fileID: string) {
    try {
      setLoading(true);
      await mutateImageSetPrimaryFile(props.image.id, fileID);
    } catch (e) {
      Toast.error(e);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Accordion defaultActiveKey={props.image.visual_files[0].id}>
      {deletingFile && (
        <DeleteFilesDialog
          onClose={() => setDeletingFile(undefined)}
          selected={[deletingFile]}
        />
      )}
      {props.image.visual_files.map((file, index) => (
        <Card key={file.id} className="image-file-card">
          <Accordion.Toggle as={Card.Header} eventKey={file.id}>
            <TruncatedText text={TextUtils.fileNameFromPath(file.path)} />
          </Accordion.Toggle>
          <Accordion.Collapse eventKey={file.id}>
            <Card.Body>
              <FileInfoPanel
                file={file}
                image={index === 0 ? props.image : undefined}
                primary={index === 0}
                ofMany
                onSetPrimaryFile={() => onSetPrimaryFile(file.id)}
                onDeleteFile={() => setDeletingFile(file)}
                loading={loading}
              />
            </Card.Body>
          </Accordion.Collapse>
        </Card>
      ))}
    </Accordion>
  );
};
