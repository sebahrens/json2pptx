package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// go-slide-creator-sx53. Closure audit of go-slide-creator-utej
// ("reduce_items/resize_list refuse fact-bearing drops"): the guard covered
// pattern arrays and, later, bullet lists — but not bullet_groups.
// reduce_text{max_items: 3} on four groups returned applied:true and silently
// deleted the only group carrying a number, one fix kind over from the bug the
// original bead was filed for.

func groupsWith(headersAndBullets ...[2]string) *deckinput.BulletGroupsInput {
	g := &deckinput.BulletGroupsInput{}
	for _, hb := range headersAndBullets {
		g.Groups = append(g.Groups, deckinput.BulletGroupInput{Header: hb[0], Bullets: []string{hb[1]}})
	}
	return g
}

func TestReduceTextRefusesDroppingAFactBearingGroup(t *testing.T) {
	ci := &ContentInput{Type: "bullet_groups", BulletGroupsValue: groupsWith(
		[2]string{"A", "one"}, [2]string{"B", "two"}, [2]string{"C", "three"},
		[2]string{"D", "Churn rose to 4% in SMB"},
	)}
	changed, refusal := reduceContentItem(ci, reduceTextBudget{maxItems: 3})

	if refusal == nil {
		t.Fatalf("dropping the only group with a number was allowed (changed=%v); groups now %d",
			changed, len(ci.BulletGroupsValue.Groups))
	}
	if refusal.Code != "semantic_review_required" {
		t.Errorf("code = %q, want semantic_review_required", refusal.Code)
	}
	if refusal.Applied {
		t.Error("a refusal must not report applied")
	}
	if !strings.Contains(refusal.Message, "confirm_semantic_change") {
		t.Errorf("message does not say how to proceed: %q", refusal.Message)
	}
	if len(ci.BulletGroupsValue.Groups) != 4 {
		t.Errorf("groups were trimmed despite the refusal: %d left", len(ci.BulletGroupsValue.Groups))
	}
}

func TestReduceTextDropsGroupsWhenConfirmed(t *testing.T) {
	ci := &ContentInput{Type: "bullet_groups", BulletGroupsValue: groupsWith(
		[2]string{"A", "one"}, [2]string{"B", "two"}, [2]string{"C", "three"},
		[2]string{"D", "Churn rose to 4% in SMB"},
	)}
	changed, refusal := reduceContentItem(ci, reduceTextBudget{maxItems: 3, confirm: true})
	if refusal != nil {
		t.Fatalf("confirm_semantic_change should apply the drop, got %+v", refusal)
	}
	if !changed || len(ci.BulletGroupsValue.Groups) != 3 {
		t.Errorf("changed=%v, %d groups left, want 3", changed, len(ci.BulletGroupsValue.Groups))
	}
}

func TestReduceTextStillDropsFactFreeGroups(t *testing.T) {
	ci := &ContentInput{Type: "bullet_groups", BulletGroupsValue: groupsWith(
		[2]string{"Alpha", "one point"}, [2]string{"Beta", "another point"},
		[2]string{"Gamma", "a third point"}, [2]string{"Delta", "a plain sentence here"},
	)}
	changed, refusal := reduceContentItem(ci, reduceTextBudget{maxItems: 3})
	if refusal != nil {
		t.Fatalf("a fact-free drop must still apply, got %+v", refusal)
	}
	if !changed || len(ci.BulletGroupsValue.Groups) != 3 {
		t.Errorf("changed=%v, %d groups left, want 3", changed, len(ci.BulletGroupsValue.Groups))
	}
}

// TestGuardDroppedGroupsReadsEveryField: a fact can live in the group label or
// body, not only its bullets.
func TestGuardDroppedGroupsReadsEveryField(t *testing.T) {
	fields := []struct {
		name string
		set  func(*deckinput.BulletGroupInput)
	}{
		{"header", func(g *deckinput.BulletGroupInput) { g.Header = "Churn 4%" }},
		{"body", func(g *deckinput.BulletGroupInput) { g.Body = "Churn 4%" }},
		{"group_label", func(g *deckinput.BulletGroupInput) { g.GroupLabel = "Churn 4%" }},
		{"bullets", func(g *deckinput.BulletGroupInput) { g.Bullets = []string{"Churn 4%"} }},
	}
	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			groups := []deckinput.BulletGroupInput{
				{Header: "A", Bullets: []string{"one"}},
				{Header: "B", Bullets: []string{"two"}},
				{Header: "C", Bullets: []string{"three"}},
				{},
			}
			f.set(&groups[3])
			if guardDroppedGroups(groups, reduceTextBudget{maxItems: 3}) == nil {
				t.Errorf("a number in %s was dropped without a refusal", f.name)
			}
		})
	}
}

func TestGuardDroppedGroupsNoOpCases(t *testing.T) {
	groups := []deckinput.BulletGroupInput{{Header: "A"}, {Header: "B"}}
	if guardDroppedGroups(groups, reduceTextBudget{maxItems: 3}) != nil {
		t.Error("nothing is dropped when the list is already within budget")
	}
	if guardDroppedGroups(groups, reduceTextBudget{}) != nil {
		t.Error("no max_items means no group truncation to guard")
	}
}
