import { before } from "src/patch";

// Copyright bulk Character assignment should behave like the other multi-select
// metadata controls: choosing one item must not close the menu.
before("PerformerSelect", (props: Record<string, unknown>) => [
  props.isMulti
    ? {
        ...props,
        closeMenuOnSelect: false,
      }
    : props,
]);
