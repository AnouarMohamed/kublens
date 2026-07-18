# KubeLens Design Context

## Design Register

Product UI. Design serves fast incident work, repeated review, and high-trust operational decisions.

## Scene Sentence

An on-call SRE scans KubeLens on a 27-inch monitor during a late-night incident, with Slack, Grafana, and kubectl nearby, needing confidence fast without visual noise.

## Color Strategy

Restrained by default. Use tinted dark neutrals for the shell, one primary accent for selection and actions, and semantic colors only for state, severity, or evidence quality.

## Typography

Use the existing product sans stack. Keep labels compact, data legible, and headings proportional to their container. Do not use hero-scale type inside dashboards, panels, tables, or modals.

## Layout Principles

- The first screen should expose the incident and change-risk workflow, not a marketing summary.
- Navigation should make the primary workbench obvious and demote secondary inventory views to drilldowns.
- Panels should be dense but ordered: risk summary, evidence, simulation, remediation, audit.
- Avoid nested cards and repeated identical card grids.
- Use skeletons for loading and clear recovery actions for errors.

## Interaction Principles

- High-risk actions show prerequisites, blockers, expected impact, and rollback context before execution.
- Simulation results show confidence and limitations next to the verdict.
- ML-backed output identifies model version, feature completeness, and whether shadow mode or blending produced the score.
- Every empty state should tell the operator what signal is missing or what action will produce data.

## Accessibility And Responsive Defaults

- Target WCAG AA contrast.
- Keep keyboard focus visible on every command.
- Use standard controls for filters, tabs, toggles, selects, sliders, and action menus.
- Tables must remain scannable on desktop and collapse into prioritized rows on narrow screens.
