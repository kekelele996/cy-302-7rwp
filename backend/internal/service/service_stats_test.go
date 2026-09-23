package service

import (
	"testing"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestAggregateKnowledgePoints(t *testing.T) {
	// Paper layout:
	// eq1 -> q1 single, KP=A, 10 points
	// eq2 -> q2 short_answer, KP=A, 10 points
	// eq3 -> q3 single, KP=B, 10 points
	// eq4 -> q4 short_answer, KP=C, 20 points
	// eq5 -> q5 true_false, KP=D, 10 points (no valid answers from anyone)
	items := []model.ExamQuestion{
		{ID: 1, ExamID: 1, QuestionID: 101, Score: 10, SortOrder: 1},
		{ID: 2, ExamID: 1, QuestionID: 102, Score: 10, SortOrder: 2},
		{ID: 3, ExamID: 1, QuestionID: 103, Score: 10, SortOrder: 3},
		{ID: 4, ExamID: 1, QuestionID: 104, Score: 20, SortOrder: 4},
		{ID: 5, ExamID: 1, QuestionID: 105, Score: 10, SortOrder: 5},
	}
	questions := map[uint]model.Question{
		101: {ID: 101, Type: constants.QuestionSingle, KnowledgePoint: "A"},
		102: {ID: 102, Type: constants.QuestionShortAnswer, KnowledgePoint: "A"},
		103: {ID: 103, Type: constants.QuestionSingle, KnowledgePoint: "B"},
		104: {ID: 104, Type: constants.QuestionShortAnswer, KnowledgePoint: "C"},
		105: {ID: 105, Type: constants.QuestionTrueFalse, KnowledgePoint: "D"},
	}

	tests := []struct {
		name    string
		answers []model.Answer
		want    []kpExpect
	}{
		{
			name: "mixed mastery, graded subjective scores drive the rate",
			answers: []model.Answer{
				// Attempt 1: KP A objective correct (10) + subjective 6/10 => 16/20 = 80
				{AttemptID: 1, ExamQuestionID: 1, QuestionID: 101, AnswerText: `"A"`, IsCorrect: boolPtr(true), Score: 10},
				{AttemptID: 1, ExamQuestionID: 2, QuestionID: 102, AnswerText: `"x"`, Score: 6},
				// KP B correct => 10/10 = 100
				{AttemptID: 1, ExamQuestionID: 3, QuestionID: 103, AnswerText: `"A"`, IsCorrect: boolPtr(true), Score: 10},
				// KP C subjective 2/20 = 10
				{AttemptID: 1, ExamQuestionID: 4, QuestionID: 104, AnswerText: `"x"`, Score: 2},
				// KP D blank -> not a participant
				{AttemptID: 1, ExamQuestionID: 5, QuestionID: 105, AnswerText: "", IsCorrect: nil, Score: 0},

				// Attempt 2: KP A objective wrong (0) + subjective 0/10 => 0/20 = 0; pooled A = 16/40 = 40
				{AttemptID: 2, ExamQuestionID: 1, QuestionID: 101, AnswerText: `"B"`, IsCorrect: boolPtr(false), Score: 0},
				{AttemptID: 2, ExamQuestionID: 2, QuestionID: 102, AnswerText: `"y"`, Score: 0},
				// KP B blank answer -> participates with 0/10; pooled B = 10/20 = 50
				{AttemptID: 2, ExamQuestionID: 3, QuestionID: 103, AnswerText: `"C"`, IsCorrect: boolPtr(false), Score: 0},
				// KP C ungraded 0/20; pooled C = 2/40 = 5
				{AttemptID: 2, ExamQuestionID: 4, QuestionID: 104, AnswerText: `"z"`, Score: 0},
				// KP D blank again
				{AttemptID: 2, ExamQuestionID: 5, QuestionID: 105, AnswerText: "", Score: 0},
			},
			want: []kpExpect{
				{kp: "C", questionCount: 1, participants: 2, rate: 5, objective: nil, weak: true},
				{kp: "A", questionCount: 2, participants: 2, rate: 40, objective: ptrFloat(50), weak: true},
				{kp: "B", questionCount: 1, participants: 2, rate: 50, objective: ptrFloat(50), weak: true},
			},
		},
		{
			name: "threshold 60 is not weak and purely subjective kp has null accuracy",
			answers: []model.Answer{
				// KP B 6/10 exactly -> 60%, not weak
				{AttemptID: 1, ExamQuestionID: 3, QuestionID: 103, AnswerText: `"A"`, IsCorrect: boolPtr(false), Score: 6},
				// KP C 12/20 exactly -> 60%, null objective accuracy
				{AttemptID: 1, ExamQuestionID: 4, QuestionID: 104, AnswerText: `"x"`, Score: 12},
			},
			want: []kpExpect{
				{kp: "B", questionCount: 1, participants: 1, rate: 60, objective: ptrFloat(0), weak: false},
				{kp: "C", questionCount: 1, participants: 1, rate: 60, objective: nil, weak: false},
			},
		},
		{
			name:    "no answers means no knowledge points",
			answers: nil,
			want:    []kpExpect{},
		},
		{
			name: "only blank answers are excluded",
			answers: []model.Answer{
				{AttemptID: 1, ExamQuestionID: 5, QuestionID: 105, AnswerText: "", Score: 0},
			},
			want: []kpExpect{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := aggregateKnowledgePoints(items, questions, tt.answers)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d kps (%+v), want %d", len(got), got, len(tt.want))
			}
			for i, w := range tt.want {
				g := got[i]
				if g.KnowledgePoint != w.kp {
					t.Errorf("[%d] kp = %q, want %q", i, g.KnowledgePoint, w.kp)
				}
				if g.QuestionCount != w.questionCount {
					t.Errorf("[%s] question count = %d, want %d", w.kp, g.QuestionCount, w.questionCount)
				}
				if g.ParticipantCount != w.participants {
					t.Errorf("[%s] participants = %d, want %d", w.kp, g.ParticipantCount, w.participants)
				}
				if g.AverageScoreRate != w.rate {
					t.Errorf("[%s] rate = %v, want %v", w.kp, g.AverageScoreRate, w.rate)
				}
				if g.Weak != w.weak {
					t.Errorf("[%s] weak = %v, want %v", w.kp, g.Weak, w.weak)
				}
				if w.objective == nil {
					if g.ObjectiveAccuracy != nil {
						t.Errorf("[%s] objective accuracy = %v, want nil", w.kp, *g.ObjectiveAccuracy)
					}
					continue
				}
				if g.ObjectiveAccuracy == nil {
					t.Errorf("[%s] objective accuracy = nil, want %v", w.kp, *w.objective)
				} else if *g.ObjectiveAccuracy != *w.objective {
					t.Errorf("[%s] objective accuracy = %v, want %v", w.kp, *g.ObjectiveAccuracy, *w.objective)
				}
			}
		})
	}
}

type kpExpect struct {
	kp            string
	questionCount int
	participants  int
	rate          float64
	objective     *float64
	weak          bool
}

func ptrFloat(v float64) *float64 {
	return &v
}
