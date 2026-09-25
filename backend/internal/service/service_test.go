package service

import (
	"testing"
	"time"

	"safetyplatform/internal/constants"
	"safetyplatform/internal/model"
)

func TestU64(t *testing.T) {
	if u64(0) != "0" || u64(123) != "123" {
		t.Error("u64 mismatch")
	}
}

func TestContains(t *testing.T) {
	if !contains(constants.UserRoleValues, constants.RoleInspector) {
		t.Error("inspector should be in roles")
	}
	if contains(constants.UserRoleValues, "bogus") {
		t.Error("bogus should not be in roles")
	}
}

func TestIncidentCategories(t *testing.T) {
	found := false
	for _, c := range constants.IncidentCategories {
		if c == "坠落" {
			found = true
		}
	}
	if !found {
		t.Error("坠落 should be in categories")
	}
}

func TestRectificationStatusValues(t *testing.T) {
	for _, s := range []string{constants.RectificationPending, constants.RectificationSubmitted, constants.RectificationReturned, constants.RectificationApproved} {
		if !constants.IsValidRectificationStatus(s) {
			t.Errorf("%s should be valid rectification status", s)
		}
	}
	if constants.IsValidRectificationStatus("bogus") {
		t.Error("bogus should not be valid rectification status")
	}
}

func TestIsOverdue(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	cases := []struct {
		name string
		task model.RectificationTask
		want bool
	}{
		{"pending past deadline is overdue", model.RectificationTask{Status: constants.RectificationPending, Deadline: &past}, true},
		{"submitted past deadline is overdue", model.RectificationTask{Status: constants.RectificationSubmitted, Deadline: &past}, true},
		{"returned past deadline is overdue", model.RectificationTask{Status: constants.RectificationReturned, Deadline: &past}, true},
		{"approved past deadline not overdue", model.RectificationTask{Status: constants.RectificationApproved, Deadline: &past}, false},
		{"pending future deadline not overdue", model.RectificationTask{Status: constants.RectificationPending, Deadline: &future}, false},
		{"pending no deadline not overdue", model.RectificationTask{Status: constants.RectificationPending}, false},
	}
	for _, tc := range cases {
		if got := isOverdue(tc.task, now); got != tc.want {
			t.Errorf("%s: isOverdue=%v want %v", tc.name, got, tc.want)
		}
	}
}
