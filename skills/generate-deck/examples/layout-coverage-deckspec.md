# Worked DeckSpec: coherent layout coverage

This example covers midnight-blue's seven core native layouts inside a two-chapter story. The nine
slides serve the narrative; `required_layouts` is a planning constraint rather than the outline.

```yaml
meta:
  title: "The state of agentic AI — mid 2026"
  archetype: market_analysis
  template: midnight-blue
  required_layouts: [title, content, two-column, section, blank-title, blank-canvas, closing]
  accent_strategy: section-keyed
  chrome:
    section_crumb: true
    page_numbers: {format: "{current} / {total}", skip: [title, closing]}

structure:
  cover:
    kind: title
    title: "The state of agentic AI — mid 2026"
    subtitle: "From copilots to supervised operators"
  auto_agenda: true
  sections:
    - title: "Where the market is"
      slides:
        - kind: table
          title: "Adoption is broad, but autonomy remains bounded"
          headers: ["Operating mode", "Typical scope", "Control model"]
          rows:
            - ["Copilot", "Single task", "Human accepts each result"]
            - ["Workflow agent", "Multi-step process", "Approval at checkpoints"]
            - ["Operator", "Persistent objective", "Policy and exception review"]
          takeaway: "The center of gravity is supervised workflow autonomy."
          source: "Illustrative synthesis, September 2026"
        - kind: comparison
          title: "Three operating models coexist"
          columns:
            - {title: "Copilot", items: ["Fast assistance inside an existing tool."]}
            - {title: "Agent", items: ["Plans and executes across connected tools."]}
          takeaway: "Value rises with autonomy, along with the cost of weak controls."
    - title: "What leaders should do"
      slides:
        - kind: kpi_snapshot
          title: "Readiness is a portfolio question"
          kpis:
            - {label: "Workflow fit", value: "High"}
            - {label: "Data access", value: "Scoped"}
            - {label: "Human control", value: "Explicit"}
          takeaway: "Scale the use cases whose control model is already clear."
        - kind: process
          title: "Move from sandbox to service in four gates"
          steps:
            - {label: "Bound the task", description: "Define the allowed outcome, data, and tools."}
            - {label: "Instrument actions", description: "Record decisions, tool calls, and policy checks."}
            - {label: "Test exceptions", description: "Probe recovery paths and approval boundaries."}
            - {label: "Scale gradually", description: "Expand autonomy only when evidence supports it."}
          takeaway: "Operational evidence should unlock each increase in autonomy."
  closing:
    kind: closing
    title: "Build the control plane before the agent fleet"
    subtitle: "Discussion"
```

Run `validate_deck_spec`, then `explain_deck_spec`. The explanation must show all seven requested
layouts assigned and no missing coverage. Render the deck, inspect every current-revision slide, and
reject it if chapter continuity, hierarchy, whitespace, visual variety, or evidence credibility is
weak even when no element clips.
