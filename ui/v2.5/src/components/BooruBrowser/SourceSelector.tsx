import { type MouseEvent, useState } from "react";
import { Dropdown } from "react-bootstrap";
import { faArrowDown, faArrowUp } from "@fortawesome/free-solid-svg-icons";
import { BooruIconButton } from "./BooruControls";
import { SourceIcon } from "./SourceIcon";
import { type BooruSource } from "./browser";

export function SourceSelector({
  sources,
  selected,
  order,
  onOrder,
  onSelect,
}: {
  sources: BooruSource[];
  selected?: BooruSource;
  order: string[];
  onOrder: (ids: string[]) => void;
  onSelect: (source: BooruSource) => void;
}) {
  const [show, setShow] = useState(false);
  const ordered = [...sources].sort((a, b) => {
    const ai = order.indexOf(a.id),
      bi = order.indexOf(b.id);
    return (ai < 0 ? order.length : ai) - (bi < 0 ? order.length : bi);
  });
  function move(index: number, direction: number) {
    const ids = ordered.map((source) => source.id);
    [ids[index], ids[index + direction]] = [ids[index + direction], ids[index]];
    onOrder(ids);
  }
  return (
    <Dropdown
      className="btn-group booru-source-selector"
      show={show}
      onToggle={setShow}
    >
      <Dropdown.Toggle
        variant="secondary"
        id="booru-sources"
        aria-label="Source"
        active={show}
      >
        {selected ? (
          <>
            <SourceIcon key={selected.instance} source={selected} />{" "}
            {selected.name}
          </>
        ) : (
          "Choose source"
        )}
      </Dropdown.Toggle>
      <Dropdown.Menu renderOnMount>
        {ordered.map((source, index) => (
          <div className="booru-source-option" key={source.id}>
            <Dropdown.Item
              role="menuitemradio"
              aria-label={source.name}
              aria-checked={selected?.id === source.id}
              active={selected?.id === source.id}
              onClick={() => {
                onSelect(source);
                setShow(false);
              }}
            >
              <SourceIcon source={source} /> {source.name}
            </Dropdown.Item>
            <BooruIconButton
              icon={faArrowUp}
              label={`Move ${source.name} up`}
              disabled={index === 0}
              onClick={(event: MouseEvent<HTMLButtonElement>) => {
                event.stopPropagation();
                move(index, -1);
              }}
            />
            <BooruIconButton
              icon={faArrowDown}
              label={`Move ${source.name} down`}
              disabled={index === ordered.length - 1}
              onClick={(event: MouseEvent<HTMLButtonElement>) => {
                event.stopPropagation();
                move(index, 1);
              }}
            />
          </div>
        ))}
      </Dropdown.Menu>
    </Dropdown>
  );
}
