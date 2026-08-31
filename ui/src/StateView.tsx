import type {
  WorkflowState,
} from "./types";

interface Props {
  state: WorkflowState;
}

export function StateView({
  state,
}: Props) {

  return (
    <pre className="state-view">
      {JSON.stringify(
        state,
        null,
        2,
      )}
    </pre>
  );
}