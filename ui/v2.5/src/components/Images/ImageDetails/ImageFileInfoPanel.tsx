import React, { useState } from "react";
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

interface IFileInfoPanelProps {
  file: GQL.ImageFileDataFragment | GQL.VideoFileDataFragment;
  image?: GQL.ImageDataFragment;
  primary?: boolean;
  ofMany?: boolean;
  onSetPrimaryFile?: () => void;
  onDeleteFile?: () => void;
  loading?: boolean;
}

function imageFormatLabel(file: GQL.ImageFileDataFragment): string {
  const format = file.format.toLowerCase();

  if (file.frame_count > 1) {
    switch (format) {
      case "png":
      case "apng":
        return "apng";
      case "jxl":
      case "jpegxl":
      case "ajxl":
        return "ajxl";
      case "webp":
      case "awebp":
        return "awebp";
      // GIF is already specifically known as an animation format, so it keeps
      // its normal name instead of becoming "agif".
      case "gif":
        return "gif";
    }
  }

  // FFmpeg reports JPEG XL as jpegxl; use the normal file-format spelling in
  // the details panel so the animated form naturally reads as ajxl.
  return format === "jpegxl" ? "jxl" : format;
}

const FileInfoPanel: React.FC<IFileInfoPanelProps> = (
  props: IFileInfoPanelProps
) => {
  const intl = useIntl();
  const checksum = props.file.fingerprints.find((f) => f.type === "md5");
  const phash = props.file.fingerprints.find((f) => f.type === "phash");
  const imageFormat =
    "format" in props.file ? imageFormatLabel(props.file) : undefined;

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
        <TextField id="media_info.md5" value={checksum?.value} truncate />
        <URLField
          id="media_info.phash"
          abbr={intl.formatMessage({ id: "media_info.phash_meaning" })}
          value={phash?.value}
          url={NavUtils.makeImagesPHashMatchUrl(phash?.value)}
          target="_self"
          truncate
          internal
        />
        <TextField id="path">
          <span className="d-flex align-items-center">
            <TruncatedText text={props.file.path} />
            <RevealInFilesystemButton fileId={props.file.id} />
          </span>
        </TextField>
        <TextField id="filesize">
          <span className="text-truncate">
            <FileSize size={props.file.size} />
          </span>
        </TextField>
        {props.image ? (
          <>
            <URLsField id="urls" urls={props.image.urls} truncate />
            <TextField
              id="created_at"
              value={TextUtils.formatDateTime(intl, props.image.created_at)}
            />
            <TextField
              id="updated_at"
              value={TextUtils.formatDateTime(intl, props.image.updated_at)}
            />
          </>
        ) : null}
        <TextField id="file_mod_time">
          <FormattedTime
            dateStyle="medium"
            timeStyle="medium"
            value={props.file.mod_time ?? 0}
          />
        </TextField>
        <TextField id="format" name="Format" value={imageFormat} />
        <TextField
          id="dimensions"
          value={`${props.file.width} x ${props.file.height}`}
          truncate
        />
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
        <URLsField id="urls" urls={props.image.urls} truncate />
        <TextField
          id="created_at"
          value={TextUtils.formatDateTime(intl, props.image.created_at)}
        />
        <TextField
          id="updated_at"
          value={TextUtils.formatDateTime(intl, props.image.updated_at)}
        />
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
