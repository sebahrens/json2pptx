package patterns

// Consulting exemplars show a complete exhibit; short authored payloads remain
// valid and can be placed beside supporting evidence in a composition.
func consultingExemplar(name string, fallback map[string]any) map[string]any {
	switch name {
	case "checklist":
		return map[string]any{"items": []any{
			map[string]any{"label": "Scope and decision rights agreed", "state": "done", "detail": "The sponsor and delivery leads signed the scope baseline, with clear owners for changes and a weekly route for resolving decisions."},
			map[string]any{"label": "Operational readiness demonstrated", "state": "done", "detail": "Service teams rehearsed the handover, confirmed support coverage and tested the recovery steps against the agreed service targets."},
			map[string]any{"label": "Independent assurance completed", "state": "open", "detail": "Risk and controls teams are reviewing the remaining evidence; each observation has an owner, a due date and a documented response."},
			map[string]any{"label": "Release approval secured", "state": "blocked", "detail": "The steering group will approve launch once assurance findings are closed and the accountable business owner accepts the residual risk."}}}
	case "prose-2col":
		return map[string]any{"columns": []any{
			map[string]any{"heading": "Demand is changing the delivery model", "paragraphs": []any{
				"Customers increasingly buy managed outcomes rather than individual products. They expect one accountable team to coordinate service quality, resolve incidents and explain performance across the full relationship. Our current operating model splits these responsibilities across several functions, creating avoidable handovers and slowing decisions.",
				"The pilot confirmed that a dedicated service lead can reduce these handovers while preserving specialist expertise. Teams resolved recurring issues earlier, shared customer evidence more consistently and made ownership visible. Scaling the approach now requires common routines, reliable management information and explicit capacity commitments from every contributing function."}},
			map[string]any{"heading": "Scale capability before entering new markets", "paragraphs": []any{
				"The next phase should establish the foundations for repeatable delivery. Appoint accountable service leads, agree the critical measures and reserve specialist capacity before adding customers. A common readiness review should test the evidence for each launch and make unresolved dependencies visible to the steering group.",
				"Investment should follow demonstrated demand and operating readiness. Expand first in the segments where the pilot evidence is strongest, review performance monthly and use the learning to refine the model. This staged approach protects the customer experience while giving leaders a clear basis for deciding when broader expansion is justified."}}}}
	case "timeline-vertical":
		return map[string]any{"stops": []any{
			map[string]any{"date": "January", "title": "Agree the mandate", "detail": "Confirm the sponsor, measurable outcomes and decision rights; baseline the current service and agree the evidence required for the first investment gate."},
			map[string]any{"date": "April", "title": "Test the operating model", "detail": "Run a controlled pilot with representative customers, rehearse support handovers and review the results with delivery, operations and risk owners."},
			map[string]any{"date": "July", "title": "Resolve readiness gaps", "detail": "Close the critical assurance findings, train the service teams and confirm the capacity needed to support a dependable launch across the priority markets."},
			map[string]any{"date": "October", "title": "Scale with evidence", "detail": "Release investment in stages, monitor the agreed service measures and use customer feedback to adjust the rollout before extending the model further."}}}
	case "sources-list":
		labels := []string{"Management interviews with business sponsors and service owners", "Finance planning baseline and approved investment assumptions", "Customer research on service quality and unresolved handovers", "Operations incident review and recovery exercise observations", "Risk and controls assessment of the proposed delivery model", "Workforce capacity forecast for specialist delivery teams", "Pilot performance dashboard and agreed service target definitions", "Market demand assessment for the priority customer segments", "Independent assurance review of launch readiness evidence", "Steering group decisions and accountable owner sign-offs", "Technology dependency map and platform release calendar", "Benefits tracking methodology and quarterly reporting baseline"}
		entries := make([]any, len(labels))
		for i, label := range labels {
			entries[i] = map[string]any{"text": label + "; programme evidence pack, September 2027."}
		}
		return map[string]any{"entries": entries}
	case "progress-list":
		return map[string]any{"rows": []any{
			map[string]any{"label": "Recurring customer revenue secured under signed service agreements", "actual": 72.0, "target": 90.0, "unit": "M"},
			map[string]any{"label": "Active customers migrated onto the supported service platform", "actual": 48.0, "target": 60.0, "unit": "k"},
			map[string]any{"label": "Service contribution margin after committed delivery investment", "actual": 21.0, "target": 25.0, "unit": "%"},
			map[string]any{"label": "Priority service controls supported by reviewed assurance evidence", "actual": 81.0, "target": 95.0, "unit": "%"},
			map[string]any{"label": "Delivery colleagues accredited for the new managed service model", "actual": 180.0, "target": 240.0, "unit": ""},
			map[string]any{"label": "Customer service issues resolved within the agreed response target", "actual": 88.0, "target": 95.0, "unit": "%"}}}
	case "raci-matrix":
		activities := []string{"Agree programme scope", "Approve investment case", "Design service controls", "Deliver the pilot", "Accept operational handover", "Authorize market launch"}
		rows := make([]any, len(activities))
		for i, a := range activities {
			rows[i] = map[string]any{"activity": a, "scores": []any{"R", "A", "C", "I"}}
		}
		return map[string]any{"roles": []any{"Delivery lead", "Sponsor", "Risk owner", "Service team"}, "activities": rows}
	case "status-board":
		names := []string{"Platform delivery", "Customer migration", "Operational readiness", "Service controls", "Colleague training", "Launch assurance"}
		rows := make([]any, len(names))
		comments := []string{"Integration tests on plan", "Pilot feedback incorporated", "Support rehearsal complete", "Evidence owner appointed", "Final sessions scheduled", "Decision pack in review"}
		for i, n := range names {
			rows[i] = map[string]any{"workstream": n, "statuses": []any{"green", "amber", "green"}, "trend": "up", "comment": comments[i]}
		}
		return map[string]any{"columns": []any{"Scope", "Delivery", "Controls"}, "rows": rows}
	}
	return fallback
}
