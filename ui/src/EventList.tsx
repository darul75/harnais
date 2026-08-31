import type { GraphEvent } from "./types";

interface Props {
  events: GraphEvent[];
}

export function EventList({
  events,
}: Props) {
  return (
    <div className="events">
      {events.length === 0 && (
        <div className="empty">
          Waiting for events...
        </div>
      )}

      {events.map((event) => {

        const time =
          new Date(
            event.time,
          ).toLocaleTimeString();

        return (
          <div
            key={event.id}
            className="event-row"
          >
            <span className="event-time">
              {time}
            </span>

            <span
              className={`event-type event-${event.type}`}
            >
              {event.type}
            </span>

            {event.nodeID && (
              <span className="event-node">
                {event.nodeID}
              </span>
            )}

            {event.message && (
              <span className="event-message">
                {event.message}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}