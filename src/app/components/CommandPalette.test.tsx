import { render, screen } from "@testing-library/react";
import { within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ASSISTANT_DRAFT_KEY } from "../../features/opsassistant/constants";
import { flattenViewItems, VIEW_SECTIONS } from "../../features/viewCatalog";
import { CommandPalette } from "./CommandPalette";

const mockNavigation = vi.hoisted(() => ({
  navigateToView: vi.fn(),
  requestClusterRefresh: vi.fn(),
}));

vi.mock("../viewNavigation", () => mockNavigation);

describe("CommandPalette", () => {
  it("navigates through view results with keyboard controls", async () => {
    const user = userEvent.setup();
    const setPaletteOpen = vi.fn();

    render(
      <CommandPalette
        paletteOpen
        setPaletteOpen={setPaletteOpen}
        sections={VIEW_SECTIONS}
        searchableItems={flattenViewItems(VIEW_SECTIONS)}
      />,
    );

    await user.click(screen.getByPlaceholderText("Search views, pods, nodes, or ask the assistant…"));
    await user.keyboard("{ArrowDown}{Enter}");

    expect(mockNavigation.navigateToView).toHaveBeenCalledWith("pods");
    expect(setPaletteOpen).toHaveBeenCalledWith(false);
  });

  it("prefills the assistant draft when asking the assistant", async () => {
    const user = userEvent.setup();
    const setPaletteOpen = vi.fn();
    window.localStorage.clear();

    render(
      <CommandPalette
        paletteOpen
        setPaletteOpen={setPaletteOpen}
        sections={VIEW_SECTIONS}
        searchableItems={flattenViewItems(VIEW_SECTIONS)}
      />,
    );

    await user.type(screen.getByPlaceholderText("Search views, pods, nodes, or ask the assistant…"), "why is dns slow");
    await user.click(screen.getByRole("button", { name: /Ask assistant:/i }));

    expect(window.localStorage.getItem(ASSISTANT_DRAFT_KEY)).toBe("why is dns slow");
    expect(mockNavigation.navigateToView).toHaveBeenCalledWith("assistant");
    expect(setPaletteOpen).toHaveBeenCalledWith(false);
  });

  it("triggers the refresh quick action", async () => {
    const user = userEvent.setup();
    const setPaletteOpen = vi.fn();

    render(
      <CommandPalette
        paletteOpen
        setPaletteOpen={setPaletteOpen}
        sections={VIEW_SECTIONS}
        searchableItems={flattenViewItems(VIEW_SECTIONS)}
      />,
    );

    await user.type(screen.getByPlaceholderText("Search views, pods, nodes, or ask the assistant…"), "refresh cluster");
    const quickActions = screen.getByText("Quick actions").closest("section");
    if (!quickActions) {
      throw new Error("Quick actions section not found");
    }

    await user.click(within(quickActions).getAllByRole("button")[0]);

    expect(mockNavigation.requestClusterRefresh).toHaveBeenCalled();
    expect(setPaletteOpen).toHaveBeenCalledWith(false);
  });
});
