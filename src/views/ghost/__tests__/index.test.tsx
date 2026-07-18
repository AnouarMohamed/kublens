import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import GhostMode from "..";

const mockAPI = vi.hoisted(() => ({
  getGhostTopology: vi.fn(),
  listGhostSimulations: vi.fn(),
  simulateGhostScenario: vi.fn(),
}));

vi.mock("../../../lib/api", () => ({
  api: mockAPI,
}));

vi.mock("three", () => {
  class Vector3 {
    x: number;
    y: number;
    z: number;

    constructor(x = 0, y = 0, z = 0) {
      this.x = x;
      this.y = y;
      this.z = z;
    }

    copy(value: Vector3) {
      this.x = value.x;
      this.y = value.y;
      this.z = value.z;
      return this;
    }
  }

  class Mesh {
    geometry: { dispose: () => void };
    material: { dispose: () => void };
    position = new Vector3();

    constructor(geometry: { dispose: () => void }, material: { dispose: () => void }) {
      this.geometry = geometry;
      this.material = material;
    }
  }

  return {
    Color: class {},
    Scene: class {
      background = null;
      rotation = { y: 0 };
      items: unknown[] = [];
      add(item: unknown) {
        this.items.push(item);
      }
      traverse(callback: (item: unknown) => void) {
        this.items.forEach(callback);
      }
    },
    PerspectiveCamera: class {
      aspect = 1;
      position = { set: vi.fn() };
      lookAt = vi.fn();
      updateProjectionMatrix = vi.fn();
    },
    WebGLRenderer: class {
      domElement = document.createElement("canvas");
      setPixelRatio = vi.fn();
      setSize = vi.fn();
      render = vi.fn();
      dispose = vi.fn();
    },
    AmbientLight: class {},
    DirectionalLight: class {
      position = { set: vi.fn() };
    },
    MeshStandardMaterial: class {
      dispose = vi.fn();
    },
    SphereGeometry: class {
      dispose = vi.fn();
    },
    BufferGeometry: class {
      setFromPoints() {
        return this;
      }
      dispose = vi.fn();
    },
    LineBasicMaterial: class {
      dispose = vi.fn();
    },
    Line: class {
      constructor(
        public geometry: unknown,
        public material: unknown,
      ) {}
    },
    Mesh,
    Vector3,
  };
});

describe("GhostMode", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        disconnect() {}
      },
    );
    mockAPI.getGhostTopology.mockResolvedValue({
      generatedAt: "2026-06-15T12:00:00Z",
      source: "summary-fallback",
      nodes: [
        {
          name: "node-a",
          status: "Ready",
          unschedulable: false,
          capacity: { cpuMilli: 1000, memoryBytes: 1000 },
          allocatable: { cpuMilli: 1000, memoryBytes: 1000 },
          used: { cpuMilli: 100, memoryBytes: 100 },
          headroom: { cpuMilli: 900, memoryBytes: 900 },
        },
      ],
      pods: [],
      services: [],
      ingresses: [],
      edges: [],
    });
    mockAPI.listGhostSimulations.mockResolvedValue({
      total: 0,
      items: [],
    });
    mockAPI.simulateGhostScenario.mockResolvedValue({
      id: "ghost-1",
      action: "node_drain",
      generatedAt: "2026-06-15T12:01:00Z",
      horizonSeconds: 900,
      engine: "in-memory",
      topologyHash: "abc123456789",
      confidence: 52,
      limitations: ["Topology came from summary fallback data."],
      verdict: {
        severity: "warning",
        summary: "Drain simulation can move 1 pod(s) from node-a.",
        recommendations: ["Watch destination node headroom during the maintenance window."],
      },
      frames: [
        { offsetSeconds: 0, nodes: [], pods: [], events: [] },
        {
          offsetSeconds: 900,
          nodes: [],
          pods: [],
          events: [{ kind: "pod_rescheduled", severity: "info", resource: "default/api", message: "moved" }],
        },
      ],
    });
  });

  it("loads topology and runs a node drain simulation", async () => {
    render(<GhostMode />);

    await waitFor(() => {
      expect(screen.getByDisplayValue("node-a")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /simulate drain/i }));

    await waitFor(() => {
      expect(mockAPI.simulateGhostScenario).toHaveBeenCalledWith({
        action: "node_drain",
        nodeName: "node-a",
        horizonSeconds: 900,
      });
    });
    expect(screen.getByText(/Drain simulation can move/)).toBeInTheDocument();
    expect(screen.getByText("52%")).toBeInTheDocument();
    expect(screen.getByText("in-memory")).toBeInTheDocument();
    expect(screen.getByText("pod_rescheduled")).toBeInTheDocument();
  });
});
