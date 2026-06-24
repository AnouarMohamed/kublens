import { useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { Bot, RefreshCw, type LucideIcon } from "lucide-react";
import { ASSISTANT_DRAFT_KEY } from "../../features/opsassistant/constants";
import { flattenViewItems, type ViewItem, type ViewSection } from "../../features/viewCatalog";
import { navigateToView, requestClusterRefresh } from "../viewNavigation";
import { VIEW_ICON } from "../../components/viewIcons";
import type { View } from "../../types";

type QuickActionID = "new-incident" | "refresh-cluster" | "open-diagnostics" | "view-audit" | "new-remediation";

type CommandResult =
  | {
      id: string;
      group: "Views";
      label: string;
      description: string;
      icon: LucideIcon;
      activate: () => void;
    }
  | {
      id: QuickActionID;
      group: "Quick actions";
      label: string;
      description: string;
      icon: LucideIcon;
      activate: () => void;
    }
  | {
      id: "ask-assistant";
      group: "Ask assistant";
      label: string;
      description: string;
      icon: LucideIcon;
      activate: () => void;
    };

interface CommandPaletteProps {
  paletteOpen: boolean;
  setPaletteOpen: Dispatch<SetStateAction<boolean>>;
  sections: ViewSection[];
  searchableItems: ViewItem[];
}

const QUICK_ACTIONS: Array<{
  id: QuickActionID;
  label: string;
  description: string;
  view?: View;
  icon: LucideIcon;
}> = [
  {
    id: "new-incident",
    label: "New incident",
    description: "Jump to the incident commander workflow.",
    view: "incidents",
    icon: VIEW_ICON.incidents,
  },
  {
    id: "refresh-cluster",
    label: "Refresh cluster",
    description: "Reload the active cluster workspace and visible views.",
    icon: RefreshCw,
  },
  {
    id: "open-diagnostics",
    label: "Open diagnostics",
    description: "Review the latest automated issue detection results.",
    view: "diagnostics",
    icon: VIEW_ICON.diagnostics,
  },
  {
    id: "view-audit",
    label: "View audit trail",
    description: "Inspect recent operator actions and API activity.",
    view: "audit",
    icon: VIEW_ICON.audit,
  },
  {
    id: "new-remediation",
    label: "New remediation",
    description: "Open remediation proposals and controlled execution.",
    view: "remediation",
    icon: VIEW_ICON.remediation,
  },
];

export function CommandPalette({ paletteOpen, setPaletteOpen, sections, searchableItems }: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!paletteOpen) {
      return;
    }
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [paletteOpen]);

  const viewItems = useMemo(
    () => (searchableItems.length > 0 ? searchableItems : flattenViewItems(sections)),
    [searchableItems, sections],
  );

  const normalizedQuery = query.trim().toLowerCase();

  const results = useMemo(() => {
    const matchingViews = viewItems
      .filter((item) => {
        if (normalizedQuery === "") {
          return true;
        }
        return `${item.label} ${item.description}`.toLowerCase().includes(normalizedQuery);
      })
      .map<CommandResult>((item) => ({
        id: item.id,
        group: "Views",
        label: item.label,
        description: item.description,
        icon: VIEW_ICON[item.id],
        activate: () => {
          navigateToView(item.id);
          setPaletteOpen(false);
        },
      }));

    const matchingActions: CommandResult[] = QUICK_ACTIONS.filter((action) => {
      if (normalizedQuery === "") {
        return true;
      }
      return `${action.label} ${action.description}`.toLowerCase().includes(normalizedQuery);
    }).map((action) => ({
      id: action.id,
      group: "Quick actions",
      label: action.label,
      description: action.description,
      icon: action.icon,
      activate: () => {
        if (action.view) {
          navigateToView(action.view);
        } else {
          requestClusterRefresh();
        }
        setPaletteOpen(false);
      },
    }));

    const exactViewMatch = viewItems.some(
      (item) => item.label.toLowerCase() === normalizedQuery || item.id.toLowerCase() === normalizedQuery,
    );

    const askAssistant: CommandResult[] =
      normalizedQuery !== "" && !exactViewMatch
        ? [
            {
              id: "ask-assistant",
              group: "Ask assistant",
              label: `Ask assistant: "${query.trim()}"`,
              description: "Open the assistant and prefill the composer with this prompt.",
              icon: Bot,
              activate: () => {
                window.localStorage.setItem(ASSISTANT_DRAFT_KEY, query.trim());
                navigateToView("assistant");
                setPaletteOpen(false);
              },
            },
          ]
        : [];

    return [...matchingViews, ...matchingActions, ...askAssistant];
  }, [normalizedQuery, query, setPaletteOpen, viewItems]);

  useEffect(() => {
    if (results.length === 0) {
      setSelectedIndex(0);
      return;
    }
    setSelectedIndex((current) => Math.min(current, results.length - 1));
  }, [results]);

  const groupedResults = useMemo(() => {
    const groups: Record<CommandResult["group"], CommandResult[]> = {
      Views: [],
      "Quick actions": [],
      "Ask assistant": [],
    };

    for (const result of results) {
      groups[result.group].push(result);
    }

    return groups;
  }, [results]);

  const activateSelected = () => {
    const target = results[selectedIndex];
    target?.activate();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-zinc-950/70 px-4 py-16 backdrop-blur-sm"
      onClick={() => setPaletteOpen(false)}
      role="presentation"
    >
      <div
        className="w-full max-w-[600px] overflow-hidden rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl"
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
      >
        <div className="border-b border-zinc-700 px-4 py-4">
          <input
            ref={inputRef}
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedIndex((current) => (results.length === 0 ? 0 : Math.min(current + 1, results.length - 1)));
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedIndex((current) => (results.length === 0 ? 0 : Math.max(current - 1, 0)));
                return;
              }
              if (event.key === "Enter") {
                event.preventDefault();
                activateSelected();
                return;
              }
              if (event.key === "Escape") {
                event.preventDefault();
                setPaletteOpen(false);
              }
            }}
            placeholder="Search views, pods, nodes, or ask the assistant…"
            className="field w-full"
          />
        </div>

        <div className="max-h-[420px] overflow-y-auto px-2 py-2">
          {(["Views", "Quick actions", "Ask assistant"] as const).map((group) => {
            const items = groupedResults[group];
            if (items.length === 0) {
              return null;
            }

            return (
              <section key={group} className="pb-2">
                <p className="px-2 py-2 text-[10px] uppercase tracking-[0.24em] text-zinc-500">{group}</p>
                <div className="space-y-1">
                  {items.map((item) => {
                    const absoluteIndex = results.findIndex(
                      (result) => result.id === item.id && result.group === item.group,
                    );
                    const Icon = item.icon;
                    const selected = absoluteIndex === selectedIndex;

                    return (
                      <button
                        key={`${item.group}-${item.id}`}
                        onClick={item.activate}
                        className={`w-full border-l-2 px-3 py-2 text-left ${
                          selected
                            ? "border-l-[var(--accent)] bg-zinc-800"
                            : "border-l-transparent hover:bg-zinc-800/60"
                        }`}
                      >
                        <div className="flex items-start gap-3">
                          <Icon size={16} className={selected ? "text-[var(--accent)]" : "text-zinc-500"} />
                          <div className="min-w-0">
                            <p className="text-sm text-zinc-200">{item.label}</p>
                            <p className="mt-0.5 text-xs text-zinc-500">{item.description}</p>
                          </div>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </section>
            );
          })}

          {results.length === 0 && (
            <div className="px-3 py-8 text-center text-sm text-zinc-500">No commands match the current query.</div>
          )}
        </div>
      </div>
    </div>
  );
}
