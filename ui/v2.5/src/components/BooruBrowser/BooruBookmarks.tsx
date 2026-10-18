import { useState } from "react";
import { Button, Dropdown, Form } from "react-bootstrap";
import {
  faFilter,
  faSave,
  faTrash,
  faStar,
  faPalette,
  faUser,
  faCopyright,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "../Shared/Icon";
import { BooruIconButton } from "./BooruControls";
import { type BooruPost } from "./browser";
import { Modal } from "react-bootstrap";
export interface Bookmarks {
  favorites: { instance: string; id: string }[];
  searches: { name: string; search: string }[];
}
export interface BookmarkAction {
  action: "favorite" | "search" | "remove-search";
  key?: string;
  favorite?: boolean;
  name?: string;
  search?: string;
}
export function SavedSearchFilters({
  bookmarks,
  busy,
  search,
  onSave,
  onSelect,
}: {
  bookmarks?: Bookmarks;
  busy: boolean;
  search: string;
  onSave: (action: BookmarkAction) => void;
  onSelect: (search: string) => void;
}) {
  const [name, setName] = useState("");
  return (
    <Dropdown>
      <Dropdown.Toggle
        className="filter-button"
        variant="secondary"
        aria-label="Saved search filters"
        title="Saved search filters"
      >
        <Icon icon={faFilter} />
      </Dropdown.Toggle>
      <Dropdown.Menu className="booru-saved-searches">
        <Dropdown.Header>Saved search filters</Dropdown.Header>
        {bookmarks?.searches.map((item) => (
          <div key={item.name} className="d-flex align-items-center">
            <Dropdown.Item onClick={() => onSelect(item.search)}>
              {item.name}
            </Dropdown.Item>
            <BooruIconButton
              icon={faTrash}
              label={`Delete search ${item.name}`}
              disabled={busy}
              onClick={() =>
                onSave({ action: "remove-search", name: item.name })
              }
            />
          </div>
        ))}
        {!bookmarks?.searches.length && (
          <Dropdown.ItemText>No saved filters</Dropdown.ItemText>
        )}
        <Dropdown.Divider />
        <div className="px-3 pb-2">
          <Form.Control
            className="text-input"
            aria-label="Saved filter name"
            placeholder="Filter name"
            maxLength={120}
            value={name}
            onChange={(event) => setName(event.currentTarget.value)}
            onKeyDown={(event) => event.stopPropagation()}
          />
          {bookmarks?.searches.some(
            (item) => item.name.toLowerCase() === name.trim().toLowerCase()
          ) && <small>This replaces the filter with the same name.</small>}
          <Button
            className="mt-2"
            size="sm"
            disabled={busy || !bookmarks || !name.trim()}
            onClick={() =>
              onSave({ action: "search", name: name.trim(), search })
            }
          >
            <Icon icon={faSave} /> Save current search
          </Button>
        </div>
      </Dropdown.Menu>
    </Dropdown>
  );
}
export function CoreTagActions({
  post,
  onTag,
}: {
  post: BooruPost;
  onTag: (name: string) => void;
}) {
  const [category, setCategory] = useState<string>();
  const categories = [
    { name: "artist", label: "Artists", icon: faPalette },
    { name: "copyright", label: "Copyrights", icon: faCopyright },
    { name: "character", label: "Characters", icon: faUser },
  ];
  const choose = (name: string) => {
    setCategory(undefined);
    onTag(name);
  };
  return (
    <div className="booru-core-tags">
      {categories.map((item) => {
        const tags = post.tags.filter((tag) => tag.category === item.name);
        return (
          tags.length > 0 && (
            <BooruIconButton
              key={item.name}
              icon={item.icon}
              label={`${item.label} for post ${post.remoteID ?? post.id}`}
              title={`${item.label}: ${tags.map((tag) => tag.label ?? tag.name).join(", ")}`}
              onClick={() =>
                tags.length === 1
                  ? choose(tags[0].name)
                  : setCategory(item.name)
              }
            />
          )
        );
      })}
      {category && (
        <Modal show onHide={() => setCategory(undefined)}>
          <Modal.Header closeButton>
            <Modal.Title>
              Choose {categories.find((item) => item.name === category)?.label}
            </Modal.Title>
          </Modal.Header>
          <Modal.Body className="booru-core-tag-list">
            {post.tags
              .filter((tag) => tag.category === category)
              .map((tag) => (
                <Button
                  key={tag.name}
                  variant="link"
                  onClick={() => choose(tag.name)}
                >
                  {tag.label ?? tag.name}
                </Button>
              ))}
          </Modal.Body>
        </Modal>
      )}
    </div>
  );
}
export function FavoriteAction({
  post,
  saved,
  busy,
  onSave,
}: {
  post: BooruPost;
  saved: boolean;
  busy: boolean;
  onSave: (action: BookmarkAction) => void;
}) {
  return (
    <BooruIconButton
      icon={faStar}
      active={saved}
      label={`${saved ? "Remove favorite" : "Favorite"} post ${post.remoteID ?? post.id}`}
      disabled={busy || !post.mediaKey}
      onClick={() =>
        onSave({ action: "favorite", key: post.mediaKey, favorite: !saved })
      }
    />
  );
}
