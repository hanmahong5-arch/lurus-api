package savings

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"
)

// TestOpportunity_RejectBranches exercises every early "(zero, false)" return of
// opportunity, so a customer-facing scenario never fabricates an opportunity for
// a missing/self/free/dearer substitute.
func TestOpportunity_RejectBranches(t *testing.T) {
	ratio_setting.InitRatioSettings()

	base := ModelSpend{Model: "gpt-4o", TotalQuota: 1000, PromptTokens: 100, CompletionTokens: 200}

	cases := []struct {
		name    string
		spend   ModelSpend
		sub     string
		mRatio  float64 // source model ratio passed by Compute
		cRatio  float64 // source completion ratio
		wantOK  bool
		wantErr string
	}{
		{
			name:    "empty substitute",
			spend:   base,
			sub:     "",
			mRatio:  1.25,
			cRatio:  4,
			wantOK:  false,
			wantErr: "empty sub must yield no opportunity",
		},
		{
			name:    "self substitute",
			spend:   base,
			sub:     "gpt-4o",
			mRatio:  1.25,
			cRatio:  4,
			wantOK:  false,
			wantErr: "self sub must yield no opportunity",
		},
		{
			name:    "unmapped substitute ratio",
			spend:   base,
			sub:     "totally-unknown-model-xyz",
			mRatio:  1.25,
			cRatio:  4,
			wantOK:  false,
			wantErr: "unmapped sub (GetModelRatio ok=false) must yield no opportunity",
		},
		{
			// Source is priced cheap (0.075) but the substitute (gpt-4o, 1.25) is
			// dearer → repriceModel floors at 0 → not an opportunity.
			name:    "substitute more expensive",
			spend:   ModelSpend{Model: "gpt-4o-mini", TotalQuota: 1000, PromptTokens: 100, CompletionTokens: 200},
			sub:     "gpt-4o",
			mRatio:  0.075,
			cRatio:  4,
			wantOK:  false,
			wantErr: "dearer sub must yield no opportunity",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, ok := opportunity(tc.spend, tc.sub, tc.mRatio, tc.cRatio)
			if ok != tc.wantOK {
				t.Fatalf("%s: got ok=%v, want %v (op=%+v)", tc.wantErr, ok, tc.wantOK, op)
			}
			if !ok && op != (Opportunity{}) {
				t.Fatalf("rejected opportunity must be zero-value, got %+v", op)
			}
		})
	}
}

// TestOpportunity_SuccessPct proves the success path returns the exact re-priced
// savings and a percentage backed out of real quota — the number a customer sees.
func TestOpportunity_SuccessPct(t *testing.T) {
	ratio_setting.InitRatioSettings()

	// gpt-4o (ratio 1.25, completion 4) → gpt-4o-mini (ratio 0.075, completion 4).
	//   effOrig = (100 + 200*4)*1.25 = 1125 ; effAlt = 900*0.075 = 67.5
	//   saved   = 1000*(1 - 67.5/1125) = 940 ; pct = 940/1000 = 0.94.
	s := ModelSpend{Model: "gpt-4o", TotalQuota: 1000, PromptTokens: 100, CompletionTokens: 200}
	op, ok := opportunity(s, "gpt-4o-mini", 1.25, 4)
	if !ok {
		t.Fatal("expected an opportunity for a cheaper same-family sub")
	}
	if op.SavingsQuota != 940 {
		t.Errorf("SavingsQuota = %d, want 940", op.SavingsQuota)
	}
	if op.CurrentQuota != 1000 || op.Model != "gpt-4o" || op.AltModel != "gpt-4o-mini" {
		t.Errorf("opportunity fields mismatch: %+v", op)
	}
	if op.SavingsPct != 0.94 {
		t.Errorf("SavingsPct = %v, want 0.94", op.SavingsPct)
	}
}

// TestTopN_SortAndTruncate covers the comparator's savings-desc ordering, the
// model-name tie-break, and the 10-item cap so the "top opportunities" list a
// customer sees is stable and bounded.
func TestTopN_SortAndTruncate(t *testing.T) {
	// 12 opportunities: descending savings 12..1, plus a tie at savings=5 to
	// exercise the model-name tie-break branch.
	var opps []Opportunity
	for i := 12; i >= 1; i-- {
		opps = append(opps, Opportunity{Model: modelName(i), SavingsQuota: int64(i)})
	}
	// Tie: two entries with SavingsQuota=5, names "m05b" (added) and existing "m05".
	opps = append(opps, Opportunity{Model: "m05b", SavingsQuota: 5})

	got := topN(opps)

	// Cap at topOpportunityLimit.
	if len(got) != topOpportunityLimit {
		t.Fatalf("topN len = %d, want %d", len(got), topOpportunityLimit)
	}
	// Strictly non-increasing by savings.
	for i := 1; i < len(got); i++ {
		if got[i-1].SavingsQuota < got[i].SavingsQuota {
			t.Fatalf("not sorted desc at %d: %d < %d", i, got[i-1].SavingsQuota, got[i].SavingsQuota)
		}
	}
	// Highest savings first; the tail (savings 1,2) must be dropped by the cap.
	if got[0].SavingsQuota != 12 {
		t.Errorf("top savings = %d, want 12", got[0].SavingsQuota)
	}
	for _, o := range got {
		if o.SavingsQuota < 3 {
			t.Errorf("cap should have dropped savings=%d", o.SavingsQuota)
		}
	}
	// Tie-break: among the two savings=5 entries, "m05" sorts before "m05b".
	var tie []string
	for _, o := range got {
		if o.SavingsQuota == 5 {
			tie = append(tie, o.Model)
		}
	}
	if len(tie) != 2 || tie[0] != "m05" || tie[1] != "m05b" {
		t.Errorf("tie-break order = %v, want [m05 m05b]", tie)
	}
}

// TestTopN_NilInput proves topN never returns a nil slice (JSON must serialize
// as [] not null).
func TestTopN_NilInput(t *testing.T) {
	got := topN(nil)
	if got == nil {
		t.Fatal("topN(nil) returned nil, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("topN(nil) len = %d, want 0", len(got))
	}
}

func modelName(i int) string {
	// Zero-padded so lexical order is stable/meaningful in the tie-break test.
	return "m" + twoDigit(i)
}

func twoDigit(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
