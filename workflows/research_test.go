package workflows

import (
	"context"
	"reflect"
	"testing"

	"harnais/agent"
	"harnais/graph"
)

func TestSplitResearchTopics(t *testing.T) {
	tests := []struct {
		name string
		plan string

		want []string
	}{
		{
			name: "three topics",
			plan: "slice internals\nslicing operations\nmemory layout",

			want: []string{
				"slice internals",
				"slicing operations",
				"memory layout",
			},
		},
		{
			name: "two topics with empty slot",
			plan: "slice internals\n\nslicing operations",

			want: []string{
				"slice internals",
				"slicing operations",
			},
		},
		{
			name: "single topic",
			plan: "slice internals",

			want: []string{
				"slice internals",
			},
		},
		{
			name: "blank plan",

			plan: "  \n\n",

			want: []string{},
		},
		{
			name: "more than three caps at three",
			plan: "a\nb\nc\nd\ne",

			want: []string{
				"a",
				"b",
				"c",
			},
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				got :=
					splitResearchTopics(test.plan)

				if !reflect.DeepEqual(
					got,
					test.want,
				) {
					t.Fatalf(
						"splitResearchTopics(%q) = %v, want %v",
						test.plan,
						got,
						test.want,
					)
				}
			},
		)
	}
}

func TestSkippableAgentSkipsEmptyState(t *testing.T) {
	inner := &agent.LoopAgent{
		AgentID: "researcher-2",
	}

	worker :=
		skipWhenEmpty(inner, "subtopic_1")

	result, err :=
		worker.Run(
			context.Background(),
			graph.WorkerInput{
				State: graph.State{
					"subtopic_1": "  ",
				},
			},
		)

	if err != nil {
		t.Fatalf(
			"skip run errored: %v",
			err,
		)
	}

	if result.State["skipped"] != true {
		t.Fatalf(
			"expected skipped=true, got %v",
			result.State,
		)
	}
}