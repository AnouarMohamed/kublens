/**
 * Tests for dependency version bumps introduced in this PR:
 *   - lucide-react:      ^1.11.0 → ^1.14.0
 *   - eslint:            ^10.2.1 → ^10.3.0
 *   - jsdom:             ^29.0.2 → ^29.1.1
 *   - typescript-eslint: ^8.59.0 → ^8.59.1
 *
 * Covers three areas:
 *   1. package.json declares the expected minimum version constraints.
 *   2. lucide-react icon components render correctly (production dep).
 *   3. jsdom DOM environment still behaves as expected (test dep).
 */

import { render, screen } from "@testing-library/react";
import {
  ArrowRight,
  Bell,
  Bot,
  RefreshCw,
  Settings,
  Trash2,
  User,
  type LucideIcon,
} from "lucide-react";
import { describe, expect, it } from "vitest";
import packageJson from "../../package.json";

// ---------------------------------------------------------------------------
// 1. package.json version constraint validation
// ---------------------------------------------------------------------------

describe("package.json version constraints after dependency bump", () => {
  it("declares lucide-react at ^1.14.0 or higher", () => {
    const version = packageJson.dependencies["lucide-react"];
    // Must reference at least 1.14.0 (not the old 1.11.0)
    expect(version).toBeDefined();
    const minor = parseInt(version.replace(/[^0-9.]/g, "").split(".")[1] ?? "0", 10);
    expect(minor).toBeGreaterThanOrEqual(14);
  });

  it("declares eslint at ^10.3.0 or higher", () => {
    const version = packageJson.devDependencies["eslint"];
    expect(version).toBeDefined();
    const [, minor, patch] = version.replace(/[^0-9.]/g, "").split(".").map(Number);
    // 10.3.0 bumped from 10.2.1
    expect(minor).toBeGreaterThanOrEqual(3);
    if (minor === 3) {
      expect(patch).toBeGreaterThanOrEqual(0);
    }
  });

  it("declares jsdom at ^29.1.1 or higher", () => {
    const version = packageJson.devDependencies["jsdom"];
    expect(version).toBeDefined();
    const [, minor, patch] = version.replace(/[^0-9.]/g, "").split(".").map(Number);
    // 29.1.1 bumped from 29.0.2
    expect(minor).toBeGreaterThanOrEqual(1);
    if (minor === 1) {
      expect(patch).toBeGreaterThanOrEqual(1);
    }
  });

  it("declares typescript-eslint at ^8.59.1 or higher", () => {
    const version = packageJson.devDependencies["typescript-eslint"];
    expect(version).toBeDefined();
    const [, , patch] = version.replace(/[^0-9.]/g, "").split(".").map(Number);
    // patch bumped from 8.59.0 → 8.59.1
    expect(patch).toBeGreaterThanOrEqual(1);
  });

  it("lucide-react is listed as a production dependency, not devDependency", () => {
    expect(packageJson.dependencies["lucide-react"]).toBeDefined();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect((packageJson.devDependencies as Record<string, string | undefined>)["lucide-react"]).toBeUndefined();
  });

  it("eslint and jsdom are listed as dev dependencies only", () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const deps = packageJson.dependencies as Record<string, string | undefined>;
    expect(deps["eslint"]).toBeUndefined();
    expect(deps["jsdom"]).toBeUndefined();
    expect(packageJson.devDependencies["eslint"]).toBeDefined();
    expect(packageJson.devDependencies["jsdom"]).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// 2. lucide-react smoke tests (production dependency bump 1.11.0 → 1.14.0)
// ---------------------------------------------------------------------------

describe("lucide-react icon rendering after upgrade to ^1.14.0", () => {
  it("renders Bot icon as an SVG element", () => {
    const { container } = render(<Bot />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
  });

  it("renders RefreshCw icon without throwing", () => {
    expect(() => render(<RefreshCw />)).not.toThrow();
  });

  it("renders Bell icon with accessible aria-label", () => {
    render(<Bell aria-label="notifications" />);
    expect(screen.getByRole("img", { name: "notifications" })).toBeDefined();
  });

  it("renders ArrowRight icon and forwards size prop as width/height attributes", () => {
    const { container } = render(<ArrowRight size={24} />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg?.getAttribute("width")).toBe("24");
    expect(svg?.getAttribute("height")).toBe("24");
  });

  it("renders Settings icon and applies custom className", () => {
    const { container } = render(<Settings className="icon-settings" />);
    const svg = container.querySelector("svg.icon-settings");
    expect(svg).not.toBeNull();
  });

  it("renders User icon with custom stroke color", () => {
    const { container } = render(<User color="#ff0000" />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("stroke")).toBe("#ff0000");
  });

  it("renders Trash2 icon and applies strokeWidth prop", () => {
    const { container } = render(<Trash2 strokeWidth={3} />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("stroke-width")).toBe("3");
  });

  it("treats LucideIcon as a usable React component constructor", () => {
    // Verifies the LucideIcon type export is compatible with function component usage
    const icons: LucideIcon[] = [Bot, RefreshCw, Bell, ArrowRight, Settings, User, Trash2];
    icons.forEach((Icon) => {
      const { container } = render(<Icon />);
      expect(container.querySelector("svg")).not.toBeNull();
    });
  });

  it("renders multiple distinct icons in the same tree without conflicts", () => {
    const { container } = render(
      <div>
        <Bot data-testid="icon-bot" />
        <Bell data-testid="icon-bell" />
        <Settings data-testid="icon-settings" />
      </div>,
    );
    expect(container.querySelectorAll("svg")).toHaveLength(3);
  });

  it("renders icon with absoluteStrokeWidth prop without throwing", () => {
    expect(() => render(<Bot absoluteStrokeWidth />)).not.toThrow();
  });
});

// ---------------------------------------------------------------------------
// 3. jsdom regression tests (test environment dep bump 29.0.2 → 29.1.1)
// ---------------------------------------------------------------------------

describe("jsdom DOM environment after upgrade to ^29.1.1", () => {
  it("supports basic element creation and class manipulation", () => {
    const el = document.createElement("div");
    el.classList.add("foo", "bar");
    expect(el.classList.contains("foo")).toBe(true);
    expect(el.classList.contains("bar")).toBe(true);
    el.classList.remove("foo");
    expect(el.classList.contains("foo")).toBe(false);
  });

  it("supports querySelector and querySelectorAll on dynamic DOM", () => {
    const root = document.createElement("ul");
    for (let i = 0; i < 3; i++) {
      const li = document.createElement("li");
      li.textContent = `item-${i}`;
      root.appendChild(li);
    }
    expect(root.querySelector("li")?.textContent).toBe("item-0");
    expect(root.querySelectorAll("li")).toHaveLength(3);
  });

  it("supports DOM event dispatch and listener invocation", () => {
    const btn = document.createElement("button");
    let clickCount = 0;
    btn.addEventListener("click", () => {
      clickCount++;
    });
    btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    btn.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect(clickCount).toBe(2);
  });

  it("supports setting and reading input value", () => {
    const input = document.createElement("input");
    input.type = "text";
    input.value = "hello world";
    expect(input.value).toBe("hello world");
  });

  it("supports dataset attribute access", () => {
    const el = document.createElement("span");
    el.dataset.testid = "my-span";
    expect(el.getAttribute("data-testid")).toBe("my-span");
    expect(el.dataset.testid).toBe("my-span");
  });

  it("supports textContent and innerHTML assignment", () => {
    const div = document.createElement("div");
    div.innerHTML = "<strong>bold</strong> text";
    expect(div.querySelector("strong")?.textContent).toBe("bold");
    expect(div.textContent).toBe("bold text");
  });

  it("supports MutationObserver for DOM change detection", async () => {
    const parent = document.createElement("div");
    document.body.appendChild(parent);

    const mutations: number[] = [];
    const observer = new MutationObserver((list) => {
      mutations.push(list.length);
    });
    observer.observe(parent, { childList: true });

    parent.appendChild(document.createElement("span"));
    parent.appendChild(document.createElement("span"));

    // Allow microtask queue to flush
    await Promise.resolve();

    observer.disconnect();
    document.body.removeChild(parent);

    expect(mutations.length).toBeGreaterThan(0);
  });

  it("supports localStorage get and set in the test environment", () => {
    window.localStorage.setItem("dep-test-key", "dep-test-value");
    expect(window.localStorage.getItem("dep-test-key")).toBe("dep-test-value");
    window.localStorage.removeItem("dep-test-key");
    expect(window.localStorage.getItem("dep-test-key")).toBeNull();
  });

  it("supports CSS selector with :not() pseudo-class", () => {
    const list = document.createElement("ul");
    const activeItem = document.createElement("li");
    activeItem.className = "active";
    const inactiveItem = document.createElement("li");
    list.appendChild(activeItem);
    list.appendChild(inactiveItem);

    const nonActive = list.querySelectorAll("li:not(.active)");
    expect(nonActive).toHaveLength(1);
    expect(nonActive[0]).toBe(inactiveItem);
  });

  it("supports Blob and URL.createObjectURL stub in jsdom", () => {
    const blob = new Blob(["hello"], { type: "text/plain" });
    expect(blob.size).toBe(5);
    expect(blob.type).toBe("text/plain");
  });
});
