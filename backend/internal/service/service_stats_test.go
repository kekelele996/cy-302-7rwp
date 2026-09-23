package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/model"
)

func boolPtr(v bool) *bool { return &v }

func TestBuildKnowledgePointStats(t *testing.T) {
	items := []model.ExamQuestion{
		{ID: 1, ExamID: 1, QuestionID: 101, Score: 4},
		{ID: 2, ExamID: 1, QuestionID: 102, Score: 6},
		{ID: 3, ExamID: 1, QuestionID: 103, Score: 5},
		{ID: 4, ExamID: 1, QuestionID: 104, Score: 5},
		{ID: 5, ExamID: 1, QuestionID: 105, Score: 10},
	}
	questions := map[uint]model.Question{
		101: {ID: 101, Type: constants.QuestionSingle, KnowledgePoint: "代数"},
		102: {ID: 102, Type: constants.QuestionShortAnswer, KnowledgePoint: "代数"},
		103: {ID: 103, Type: constants.QuestionTrueFalse, KnowledgePoint: "几何"},
		104: {ID: 104, Type: constants.QuestionSingle, KnowledgePoint: "概率"},
		105: {ID: 105, Type: constants.QuestionShortAnswer, KnowledgePoint: "写作"},
	}
	answersByAttempt := map[uint][]model.Answer{
		11: {
			{AttemptID: 11, ExamQuestionID: 1, QuestionID: 101, AnswerText: `"A"`, IsCorrect: boolPtr(true), Score: 4},
			{AttemptID: 11, ExamQuestionID: 2, QuestionID: 102, AnswerText: `"作答内容"`, Score: 3},
			{AttemptID: 11, ExamQuestionID: 3, QuestionID: 103, AnswerText: `"T"`, IsCorrect: boolPtr(true), Score: 5},
			{AttemptID: 11, ExamQuestionID: 4, QuestionID: 104, AnswerText: ""},
			{AttemptID: 11, ExamQuestionID: 5, QuestionID: 105, AnswerText: `"作文"`, Score: 8},
		},
		12: {
			{AttemptID: 12, ExamQuestionID: 1, QuestionID: 101, AnswerText: `"B"`, IsCorrect: boolPtr(false), Score: 0},
			{AttemptID: 12, ExamQuestionID: 2, QuestionID: 102, AnswerText: "  "},
			{AttemptID: 12, ExamQuestionID: 3, QuestionID: 103, AnswerText: `"T"`, IsCorrect: boolPtr(true), Score: 5},
		},
	}

	stats := buildKnowledgePointStats(items, questions, answersByAttempt)

	if len(stats) != 3 {
		t.Fatalf("expected 3 knowledge points, got %d: %+v", len(stats), stats)
	}

	byKP := make(map[string]int, len(stats))
	for i, s := range stats {
		byKP[s.KnowledgePoint] = i
	}

	algebra := stats[byKP["代数"]]
	if algebra.QuestionCount != 2 {
		t.Errorf("代数 question count = %d, want 2", algebra.QuestionCount)
	}
	if algebra.ParticipantCount != 2 {
		t.Errorf("代数 participants = %d, want 2", algebra.ParticipantCount)
	}
	// (4+3+0) / (4+6+4) = 50%
	if algebra.AvgScoreRate != 50 {
		t.Errorf("代数 avg score rate = %v, want 50", algebra.AvgScoreRate)
	}
	if algebra.ObjectiveAccuracy == nil || *algebra.ObjectiveAccuracy != 50 {
		t.Errorf("代数 objective accuracy = %v, want 50", algebra.ObjectiveAccuracy)
	}
	if !algebra.Weak {
		t.Errorf("代数 should be weak (rate < 60)")
	}

	geometry := stats[byKP["几何"]]
	if geometry.AvgScoreRate != 100 || geometry.Weak {
		t.Errorf("几何 rate = %v weak = %v, want 100 and not weak", geometry.AvgScoreRate, geometry.Weak)
	}
	if geometry.ObjectiveAccuracy == nil || *geometry.ObjectiveAccuracy != 100 {
		t.Errorf("几何 objective accuracy = %v, want 100", geometry.ObjectiveAccuracy)
	}

	writing := stats[byKP["写作"]]
	if writing.ObjectiveAccuracy != nil {
		t.Errorf("写作 has no objective questions, accuracy should be nil, got %v", *writing.ObjectiveAccuracy)
	}
	if writing.ParticipantCount != 1 || writing.AvgScoreRate != 80 {
		t.Errorf("写作 participants = %d rate = %v, want 1 and 80", writing.ParticipantCount, writing.AvgScoreRate)
	}

	if _, ok := byKP["概率"]; ok {
		t.Errorf("概率 has no valid answers and must be excluded")
	}

	// Sorted by average score rate ascending: 代数(50) < 写作(80) < 几何(100).
	wantOrder := []string{"代数", "写作", "几何"}
	for i, kp := range wantOrder {
		if stats[i].KnowledgePoint != kp {
			t.Errorf("stats[%d] = %s, want %s (ascending by avg score rate)", i, stats[i].KnowledgePoint, kp)
		}
	}
}

func TestBuildKnowledgePointStatsEmpty(t *testing.T) {
	stats := buildKnowledgePointStats(nil, nil, nil)
	if len(stats) != 0 {
		t.Fatalf("expected no stats, got %+v", stats)
	}
}

// fakeStatsRepo is an in-memory StatsRepo for service-level tests.
type fakeStatsRepo struct {
	exam      *model.Exam
	attempts  []model.ExamAttempt
	items     []model.ExamQuestion
	questions map[uint]model.Question
	answers   map[uint][]model.Answer
}

func (f *fakeStatsRepo) FindExamByID(context.Context, uint) (*model.Exam, error) { return f.exam, nil }
func (f *fakeStatsRepo) FindUserByID(_ context.Context, id uint) (*model.User, error) {
	return &model.User{ID: id}, nil
}
func (f *fakeStatsRepo) ListAttemptsByExam(context.Context, uint) ([]model.ExamAttempt, error) {
	return f.attempts, nil
}
func (f *fakeStatsRepo) ListExamQuestions(context.Context, uint) ([]model.ExamQuestion, error) {
	return f.items, nil
}
func (f *fakeStatsRepo) ListAnswersByAttempt(_ context.Context, attemptID uint) ([]model.Answer, error) {
	return f.answers[attemptID], nil
}
func (f *fakeStatsRepo) FindQuestionsByIDs(context.Context, []uint) (map[uint]model.Question, error) {
	return f.questions, nil
}
func (f *fakeStatsRepo) CountUsers(context.Context) (int64, error)     { return 0, nil }
func (f *fakeStatsRepo) CountQuestions(context.Context) (int64, error) { return 0, nil }
func (f *fakeStatsRepo) CountExams(context.Context) (int64, error)     { return 0, nil }
func (f *fakeStatsRepo) CountAttempts(context.Context) (int64, error)  { return 0, nil }

func TestExamStatsReflectsRegrading(t *testing.T) {
	repo := &fakeStatsRepo{
		exam: &model.Exam{ID: 1, Title: "期中考试", TotalScore: 10, CreatedBy: 7},
		attempts: []model.ExamAttempt{
			{ID: 11, ExamID: 1, StudentID: 101, Status: constants.AttemptSubmitted, ObjectiveScore: 4, TotalScore: 4},
		},
		items: []model.ExamQuestion{
			{ID: 1, ExamID: 1, QuestionID: 201, Score: 4},
			{ID: 2, ExamID: 1, QuestionID: 202, Score: 6},
		},
		questions: map[uint]model.Question{
			201: {ID: 201, Type: constants.QuestionSingle, KnowledgePoint: "代数"},
			202: {ID: 202, Type: constants.QuestionShortAnswer, KnowledgePoint: "代数"},
		},
		answers: map[uint][]model.Answer{
			11: {
				{AttemptID: 11, ExamQuestionID: 1, QuestionID: 201, AnswerText: `"A"`, IsCorrect: boolPtr(true), Score: 4},
				{AttemptID: 11, ExamQuestionID: 2, QuestionID: 202, AnswerText: `"作答"`, Score: 0},
			},
		},
	}
	svc := NewStatsService(repo, slog.Default())

	before, err := svc.ExamStats(context.Background(), constants.RoleTeacher, 7, 1)
	if err != nil {
		t.Fatalf("ExamStats() error = %v", err)
	}
	if len(before.KnowledgePoints) != 1 {
		t.Fatalf("expected 1 knowledge point, got %+v", before.KnowledgePoints)
	}
	// (4+0)/(4+6) = 40% -> weak before grading.
	if got := before.KnowledgePoints[0].AvgScoreRate; got != 40 {
		t.Fatalf("avg score rate before grading = %v, want 40", got)
	}
	if !before.KnowledgePoints[0].Weak {
		t.Fatalf("knowledge point should be weak before grading")
	}

	// Teacher grades the subjective answer: statistics must re-aggregate.
	repo.answers[11][1].Score = 6
	repo.answers[11][1].GradedBy = 7

	after, err := svc.ExamStats(context.Background(), constants.RoleTeacher, 7, 1)
	if err != nil {
		t.Fatalf("ExamStats() after grading error = %v", err)
	}
	// (4+6)/(4+6) = 100% -> no longer weak.
	if got := after.KnowledgePoints[0].AvgScoreRate; got != 100 {
		t.Fatalf("avg score rate after grading = %v, want 100", got)
	}
	if after.KnowledgePoints[0].Weak {
		t.Fatalf("knowledge point should not be weak after grading")
	}
}
