import React from "react";
import { Form } from "react-bootstrap";
import {
  CriterionValue,
  ModifierCriterion,
  Option,
} from "../../../models/list-filter/criteria/criterion";

interface IInputFilterProps {
  criterion: ModifierCriterion<CriterionValue>;
  onValueChanged: (value: string) => void;
}

export const InputFilter: React.FC<IInputFilterProps> = ({
  criterion,
  onValueChanged,
}) => {
  const { inputType, options } = criterion.modifierCriterionOption();

  function onChanged(event: React.ChangeEvent<HTMLInputElement>) {
    onValueChanged(event.target.value);
  }

  if (inputType === "select") {
    function getOptionValue(option: Option): string {
      return typeof option === "object" ? option.id : option.toString();
    }

    function getOptionLabel(option: Option): string {
      return typeof option === "object"
        ? (option.name ?? option.id)
        : option.toString();
    }

    return (
      <Form.Group>
        <Form.Control
          as="select"
          className="btn-secondary"
          onChange={(event: React.ChangeEvent<HTMLSelectElement>) =>
            onValueChanged(event.currentTarget.value)
          }
          value={criterion.value ? criterion.value.toString() : ""}
        >
          <option value="">Select format</option>
          {options?.map((option) => {
            const value = getOptionValue(option);
            return (
              <option key={value} value={value}>
                {getOptionLabel(option)}
              </option>
            );
          })}
        </Form.Control>
      </Form.Group>
    );
  }

  return (
    <Form.Group>
      <Form.Control
        className="btn-secondary"
        type={inputType}
        onChange={onChanged}
        value={criterion.value ? criterion.value.toString() : ""}
      />
    </Form.Group>
  );
};
