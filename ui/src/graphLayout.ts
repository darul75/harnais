export type LayoutNode = {
  id: string;
  nodeId: string;
  x: number;
  y: number;
};

export type LayoutEdge = {
  from: string;
  to: string;
};

export function buildColumnLayout(
  nodes: { id: string }[],
  edges: LayoutEdge[],
  columnWidth = 230,
  rowHeight = 100,
  originX = 40,
  originY = 40,
  width = 180,
): LayoutNode[] {
  if (!nodes.length) {
    return [];
  }

  const columns = new Map<string, number>();

  const indegree = new Map<string, number>();

  for (const node of nodes) {
    indegree.set(node.id, 0);
  }

  for (const edge of edges) {
    if (indegree.has(edge.to)) {
      indegree.set(
        edge.to,
        (indegree.get(edge.to) ?? 0) + 1,
      );
    }
  }

  const queue = nodes
    .filter((node) => (indegree.get(node.id) ?? 0) === 0)
    .map((node) => node.id);

  for (const id of queue) {
    columns.set(id, 0);
  }

  while (queue.length) {
    const current = queue.shift()!;

    const currentColumn = columns.get(current) ?? 0;

    for (const edge of edges) {
      if (edge.from !== current) {
        continue;
      }

      const next = edge.to;

      const nextColumn = Math.max(
        columns.get(next) ?? 0,
        currentColumn + 1,
      );

      columns.set(next, nextColumn);

      const remaining = (indegree.get(next) ?? 1) - 1;

      indegree.set(next, remaining);

      if (remaining === 0) {
        queue.push(next);
      }
    }
  }

  let maxColumn = Math.max(...columns.values(), 0);

  for (const node of nodes) {
    if (!columns.has(node.id)) {
      columns.set(node.id, ++maxColumn);
    }
  }

  const byColumn = new Map<number, { id: string }[]>();

  for (const node of nodes) {
    const column = columns.get(node.id) ?? 0;

    const items = byColumn.get(column) ?? [];

    items.push(node);

    byColumn.set(column, items);
  }

  const result: LayoutNode[] = [];

  for (const [column, columnNodes] of byColumn) {
    columnNodes.forEach((node, index) => {
      result.push({
        id: node.id,
        nodeId: node.id,
        x: originX + column * columnWidth,
        y: originY + index * rowHeight,
      });
    });
  }

  return result;
}

export function viewBoxFor(
  nodes: LayoutNode[],
  minWidth = 900,
  minHeight = 400,
  nodeWidth = 180,
  nodeHeight = 72,
) {
  if (!nodes.length) {
    return `0 0 ${minWidth} ${minHeight}`;
  }

  const maxX = Math.max(
    ...nodes.map((node) => node.x + nodeWidth),
    minWidth,
  );

  const maxY = Math.max(
    ...nodes.map((node) => node.y + nodeHeight),
    minHeight,
  );

  return `0 0 ${maxX + 20} ${maxY + 20}`;
}

export function arrowPoints(
  x1: number,
  y1: number,
  x2: number,
  y2: number,
) {
  const angle = Math.atan2(y2 - y1, x2 - x1);

  const length = 8;

  const p1x = x2 - length * Math.cos(angle);

  const p1y = y2 - length * Math.sin(angle);

  const p2x = x2 - length * Math.cos(angle - 0.5);

  const p2y = y2 - length * Math.sin(angle - 0.5);

  const p3x = x2 - length * Math.cos(angle + 0.5);

  const p3y = y2 - length * Math.sin(angle + 0.5);

  return `${p1x},${p1y} ${p2x},${p2y} ${x2},${y2} ${p3x},${p3y}`;
}