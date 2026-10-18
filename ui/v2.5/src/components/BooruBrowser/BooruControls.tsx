import { type ComponentProps } from "react";
import { Button, Dropdown } from "react-bootstrap";
import { type IconDefinition } from "@fortawesome/fontawesome-svg-core";
import { Icon } from "../Shared/Icon";

export function BooruIconButton({
  icon,
  label,
  ...props
}: {
  icon: IconDefinition;
  label: string;
} & Omit<ComponentProps<typeof Button>, "children">) {
  return (
    <Button
      variant="secondary"
      size="sm"
      className="minimal booru-icon-button"
      aria-label={label}
      title={label}
      {...props}
    >
      <Icon icon={icon} />
    </Button>
  );
}

export function BooruIconMenu({
  label,
  icon,
  value,
  options,
  onChange,
}: {
  label: string;
  icon: IconDefinition;
  value: string;
  options: { value: string; label: string; disabled?: boolean }[];
  onChange: (value: string) => void;
}) {
  return (
    <Dropdown>
      <Dropdown.Toggle
        variant="secondary"
        size="sm"
        className={`minimal booru-icon-menu ${value ? "active" : ""}`}
        aria-label={label}
        title={`${label}: ${options.find((option) => option.value === value)?.label ?? ""}`}
      >
        <Icon icon={icon} />
      </Dropdown.Toggle>
      <Dropdown.Menu aria-label={`${label} options`}>
        <Dropdown.Header>{label}</Dropdown.Header>
        {options.map((option) => (
          <Dropdown.Item
            key={option.value}
            role="menuitemradio"
            aria-checked={value === option.value}
            active={value === option.value}
            disabled={option.disabled}
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </Dropdown.Item>
        ))}
      </Dropdown.Menu>
    </Dropdown>
  );
}
