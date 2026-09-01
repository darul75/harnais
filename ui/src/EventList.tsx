import type {
  GraphEvent,
} from "./types";

interface Props {
  events: GraphEvent[];

  selectedExecutionId:
    | string
    | null;

  onSelectExecution: (
    executionId: string,
  ) => void;
}

export function EventList({
  events,
  selectedExecutionId,
  onSelectExecution,
}: Props) {

  return (
    <div className="events">

      {events.length === 0 && (
        <div className="empty">
          Waiting for events...
        </div>
      )}

      {events.map(
        (event) => {

          const time =
            new Date(
              event.time,
            ).toLocaleTimeString();

          const selected =
            event.executionID ===
            selectedExecutionId;

          const clickable =
            !!event.executionID;

          return (
            <div
              key={event.id}
              className={
                `event-row ${
                  selected
                    ? "event-selected"
                    : ""
                } ${
                  clickable
                    ? "event-clickable"
                    : ""
                }`
              }
              onClick={() => {

                if (
                  event.executionID
                ) {

                  onSelectExecution(
                    event.executionID,
                  );
                }
              }}
            >

              <span className="event-time">
                {time}
              </span>

              <span className="event-type">
                {event.type}
              </span>

              {event.nodeID && (
                <span className="event-node">
                  {event.nodeID}
                </span>
              )}

              {event.agentID && (
                <span className="event-agent">
                  {event.agentID}
                </span>
              )}

              {event.toolID && (
                <span className="event-tool">
                  {event.toolID}
                </span>
              )}

              {event.message && (
                <span className="event-message">
                  {event.message}
                </span>
              )}

            </div>
          );
        },
      )}

    </div>
  );
}