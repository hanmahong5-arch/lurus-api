package app

import (
	"testing"

	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Pure-logic tests for the Midjourney request → model-name transforms.
// These functions touch no DB / gin / network, so they are directly callable.

func TestCoverActionToModelName(t *testing.T) {
	tests := []struct {
		name   string
		action string
		want   string
	}{
		{"imagine_lowercased_with_prefix", constant.MjActionImagine, "mj_imagine"},
		{"upscale_lowercased_with_prefix", constant.MjActionUpscale, "mj_upscale"},
		{"swap_face_special_cased", constant.MjActionSwapFace, "swap_face"},
		{"arbitrary_action_gets_prefix", "FOO", "mj_foo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoverActionToModelName(tc.action); got != tc.want {
				t.Errorf("CoverActionToModelName(%q) = %q, want %q", tc.action, got, tc.want)
			}
		})
	}
}

func TestCoverPlusActionToNormalAction(t *testing.T) {
	tests := []struct {
		name       string
		customId   string
		wantErr    bool
		wantAction string
		wantIndex  int
	}{
		{"empty_custom_id_errors", "", true, "", 0},
		{"upsample_sets_upscale_and_index", "MJ::JOB::upsample::2::abc", false, constant.MjActionUpscale, 2},
		{"upsample_bad_index_errors", "MJ::JOB::upsample::x::abc", true, "", 0},
		{"variation_sets_index", "MJ::JOB::variation::3::abc", false, constant.MjActionVariation, 3},
		{"low_variation", "MJ::JOB::low_variation::1::abc", false, constant.MjActionLowVariation, 1},
		{"high_variation", "MJ::JOB::high_variation::1::abc", false, constant.MjActionHighVariation, 1},
		{"pan_left", "MJ::JOB::pan_left::1::abc", false, constant.MjActionPan, 1},
		{"reroll", "MJ::JOB::reroll::0::abc", false, constant.MjActionReRoll, 1},
		{"outpaint_maps_to_zoom", "MJ::Outpaint::50::abc", false, constant.MjActionZoom, 1},
		{"custom_zoom", "MJ::CustomZoom::50::abc", false, constant.MjActionCustomZoom, 1},
		{"inpaint", "MJ::Inpaint::region::abc", false, constant.MjActionInPaint, 1},
		{"unknown_action_errors", "MJ::JOB::teleport::1::abc", true, "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &dto.MidjourneyRequest{CustomId: tc.customId}
			resp := CoverPlusActionToNormalAction(req)
			if tc.wantErr {
				if resp == nil {
					t.Fatalf("CoverPlusActionToNormalAction(%q) = nil, want error response", tc.customId)
				}
				return
			}
			if resp != nil {
				t.Fatalf("CoverPlusActionToNormalAction(%q) = %+v, want nil (success)", tc.customId, resp)
			}
			if req.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", req.Action, tc.wantAction)
			}
			if req.Index != tc.wantIndex {
				t.Errorf("Index = %d, want %d", req.Index, tc.wantIndex)
			}
		})
	}
}

func TestConvertSimpleChangeParams(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantNil    bool
		wantAction string
		wantIndex  int
		wantTaskId string
	}{
		{"upscale_index_1", "task123 U1", false, "UPSCALE", 1, "task123"},
		{"variation_index_2", "task123 V2", false, "VARIATION", 2, "task123"},
		{"reroll_no_index", "task123 R", false, "REROLL", 0, "task123"},
		{"wrong_field_count", "task123", true, "", 0, ""},
		{"unknown_action_letter", "task123 X1", true, "", 0, ""},
		{"index_out_of_range_high", "task123 U9", true, "", 0, ""},
		{"index_below_range", "task123 U0", true, "", 0, ""},
		{"index_not_a_number", "task123 Ux", true, "", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ConvertSimpleChangeParams(tc.content)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("ConvertSimpleChangeParams(%q) = %+v, want nil", tc.content, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ConvertSimpleChangeParams(%q) = nil, want params", tc.content)
			}
			if got.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", got.Action, tc.wantAction)
			}
			if got.Index != tc.wantIndex {
				t.Errorf("Index = %d, want %d", got.Index, tc.wantIndex)
			}
			if got.TaskId != tc.wantTaskId {
				t.Errorf("TaskId = %q, want %q", got.TaskId, tc.wantTaskId)
			}
		})
	}
}

func TestGetMjRequestModel(t *testing.T) {
	tests := []struct {
		name      string
		relayMode int
		req       *dto.MidjourneyRequest
		wantModel string
		wantErr   bool
		wantOK    bool
	}{
		{"imagine", relayconstant.RelayModeMidjourneyImagine, &dto.MidjourneyRequest{}, "mj_imagine", false, true},
		{"swap_face", relayconstant.RelayModeSwapFace, &dto.MidjourneyRequest{}, "swap_face", false, true},
		{"blend", relayconstant.RelayModeMidjourneyBlend, &dto.MidjourneyRequest{}, "mj_blend", false, true},
		{"video", relayconstant.RelayModeMidjourneyVideo, &dto.MidjourneyRequest{}, "mj_video", false, true},
		{"edits", relayconstant.RelayModeMidjourneyEdits, &dto.MidjourneyRequest{}, "mj_edits", false, true},
		{"describe", relayconstant.RelayModeMidjourneyDescribe, &dto.MidjourneyRequest{}, "mj_describe", false, true},
		{"shorten", relayconstant.RelayModeMidjourneyShorten, &dto.MidjourneyRequest{}, "mj_shorten", false, true},
		{"modal", relayconstant.RelayModeMidjourneyModal, &dto.MidjourneyRequest{}, "mj_modal", false, true},
		{"upload", relayconstant.RelayModeMidjourneyUpload, &dto.MidjourneyRequest{}, "mj_upload", false, true},
		{"task_fetch_by_condition_empty", relayconstant.RelayModeMidjourneyTaskFetchByCondition, &dto.MidjourneyRequest{}, "", false, true},
		{"notify_empty", relayconstant.RelayModeMidjourneyNotify, &dto.MidjourneyRequest{}, "", false, true},
		{"change_uses_request_action", relayconstant.RelayModeMidjourneyChange, &dto.MidjourneyRequest{Action: constant.MjActionUpscale}, "mj_upscale", false, true},
		{"task_fetch_returns_empty_no_err", relayconstant.RelayModeMidjourneyTaskFetch, &dto.MidjourneyRequest{}, "", false, true},
		{"action_normalizes_custom_id", relayconstant.RelayModeMidjourneyAction, &dto.MidjourneyRequest{CustomId: "MJ::JOB::upsample::2::abc"}, "mj_upscale", false, true},
		{"action_empty_custom_id_errors", relayconstant.RelayModeMidjourneyAction, &dto.MidjourneyRequest{}, "", true, false},
		{"simple_change_valid", relayconstant.RelayModeMidjourneySimpleChange, &dto.MidjourneyRequest{Content: "task123 U1"}, "mj_upscale", false, true},
		{"simple_change_invalid_errors", relayconstant.RelayModeMidjourneySimpleChange, &dto.MidjourneyRequest{Content: "bad"}, "", true, false},
		{"unknown_relay_mode_errors", 99999, &dto.MidjourneyRequest{}, "", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			model, errResp, ok := GetMjRequestModel(tc.relayMode, tc.req)
			if model != tc.wantModel {
				t.Errorf("model = %q, want %q", model, tc.wantModel)
			}
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantErr && errResp == nil {
				t.Errorf("errResp = nil, want non-nil error response")
			}
			if !tc.wantErr && errResp != nil {
				t.Errorf("errResp = %+v, want nil", errResp)
			}
		})
	}
}
