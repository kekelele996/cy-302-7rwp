package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/gbexam/online-exam/internal/constants"
	"github.com/gbexam/online-exam/internal/dto"
	"github.com/gbexam/online-exam/internal/model"
)

// StatsService builds dashboard and exam statistics.
type StatsService struct {
	baseService
	repo StatsRepo
}

// NewStatsService constructs StatsService.
func NewStatsService(repo StatsRepo, logger *slog.Logger) *StatsService {
	return &StatsService{baseService: NewBaseService(logger), repo: repo}
}

// Overview returns aggregate counts for the dashboard.
func (s *StatsService) Overview(ctx context.Context) (*dto.OverviewResponse, error) {
	users, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	questions, err := s.repo.CountQuestions(ctx)
	if err != nil {
		return nil, fmt.Errorf("count questions: %w", err)
	}
	exams, err := s.repo.CountExams(ctx)
	if err != nil {
		return nil, fmt.Errorf("count exams: %w", err)
	}
	attempts, err := s.repo.CountAttempts(ctx)
	if err != nil {
		return nil, fmt.Errorf("count attempts: %w", err)
	}
	return &dto.OverviewResponse{
		UserCount:     users,
		QuestionCount: questions,
		ExamCount:     exams,
		AttemptCount:  attempts,
	}, nil
}

// weakScoreRate marks knowledge points mastered below this percentage as weak.
const weakScoreRate = 60.0

// ExamStats returns score statistics and ranking for one exam.
func (s *StatsService) ExamStats(ctx context.Context, role string, userID, examID uint) (*dto.ExamStatResponse, error) {
	exam, err := s.repo.FindExamByID(ctx, examID)
	if err != nil {
		return nil, err
	}
	if role == constants.RoleTeacher && exam.CreatedBy != userID {
		return nil, ErrForbidden
	}
	attempts, err := s.repo.ListAttemptsByExam(ctx, examID)
	if err != nil {
		return nil, fmt.Errorf("list attempts by exam: %w", err)
	}

	submitted := make([]model.ExamAttempt, 0, len(attempts))
	for _, a := range attempts {
		if a.Status == constants.AttemptSubmitted {
			submitted = append(submitted, a)
		}
	}
	sort.Slice(submitted, func(i, j int) bool {
		if submitted[i].TotalScore != submitted[j].TotalScore {
			return submitted[i].TotalScore > submitted[j].TotalScore
		}
		if submitted[i].SubmittedAt != nil && submitted[j].SubmittedAt != nil {
			return submitted[i].SubmittedAt.Before(*submitted[j].SubmittedAt)
		}
		return submitted[i].ID < submitted[j].ID
	})

	total := 0.0
	highest := 0.0
	lowest := 0.0
	passCount := 0
	if len(submitted) > 0 {
		highest = submitted[0].TotalScore
		lowest = submitted[len(submitted)-1].TotalScore
	}
	for _, a := range submitted {
		total += a.TotalScore
		if exam.TotalScore > 0 && a.TotalScore >= exam.TotalScore*0.6 {
			passCount++
		}
	}
	average := 0.0
	if len(submitted) > 0 {
		average = total / float64(len(submitted))
	}

	buckets := buildScoreBuckets(submitted, exam.TotalScore)
	ranking := make([]dto.RankItem, 0, len(submitted))
	for i, a := range submitted {
		name := ""
		username := ""
		if user, userErr := s.repo.FindUserByID(ctx, a.StudentID); userErr == nil {
			name = user.Name
			username = user.Username
		}
		ranking = append(ranking, dto.RankItem{
			Rank:            i + 1,
			StudentName:     name,
			StudentUsername: username,
			TotalScore:      a.TotalScore,
			SubmittedAt:     a.SubmittedAt,
		})
	}

	knowledgePoints, err := s.knowledgePointStats(ctx, submitted)
	if err != nil {
		return nil, err
	}

	return &dto.ExamStatResponse{
		ExamID:            exam.ID,
		ExamTitle:         exam.Title,
		ParticipantCount:  len(submitted),
		AverageScore:      round2(average),
		HighestScore:      highest,
		LowestScore:       lowest,
		PassCount:         passCount,
		ScoreDistribution: buckets,
		KnowledgePoints:   knowledgePoints,
		Ranking:           ranking,
	}, nil
}

// knowledgePointStats aggregates mastery by question knowledge point for submitted attempts.
func (s *StatsService) knowledgePointStats(ctx context.Context, submitted []model.ExamAttempt) ([]dto.KnowledgePointStat, error) {
	if len(submitted) == 0 {
		return []dto.KnowledgePointStat{}, nil
	}
	examID := submitted[0].ExamID
	items, err := s.repo.ListExamQuestions(ctx, examID)
	if err != nil {
		return nil, fmt.Errorf("list exam questions: %w", err)
	}
	questionIDs := make([]uint, 0, len(items))
	for _, it := range items {
		questionIDs = append(questionIDs, it.QuestionID)
	}
	questions, err := s.repo.FindQuestionsByIDs(ctx, questionIDs)
	if err != nil {
		return nil, fmt.Errorf("find questions: %w", err)
	}

	attemptIDs := make([]uint, 0, len(submitted))
	for _, a := range submitted {
		attemptIDs = append(attemptIDs, a.ID)
	}
	answers, err := s.repo.ListAnswersByAttempts(ctx, attemptIDs)
	if err != nil {
		return nil, fmt.Errorf("list answers by attempts: %w", err)
	}

	return aggregateKnowledgePoints(items, questions, answers), nil
}

// aggregateKnowledgePoints is a pure aggregation used by ExamStats and unit tests.
// Knowledge points without any valid (non-empty) answer are excluded. Results are
// ordered by average score rate ascending, then by knowledge point name.
func aggregateKnowledgePoints(
	items []model.ExamQuestion,
	questions map[uint]model.Question,
	answers []model.Answer,
) []dto.KnowledgePointStat {
	type kpMeta struct {
		questionIDs  map[uint]struct{}
		objectiveIDs map[uint]struct{}
	}
	metaByKP := map[string]*kpMeta{}
	eqByID := make(map[uint]model.ExamQuestion, len(items))
	for _, it := range items {
		eqByID[it.ID] = it
		q, ok := questions[it.QuestionID]
		if !ok || q.KnowledgePoint == "" {
			continue
		}
		meta, exists := metaByKP[q.KnowledgePoint]
		if !exists {
			meta = &kpMeta{questionIDs: map[uint]struct{}{}, objectiveIDs: map[uint]struct{}{}}
			metaByKP[q.KnowledgePoint] = meta
		}
		meta.questionIDs[q.ID] = struct{}{}
		if ObjectiveQuestionTypes()[q.Type] {
			meta.objectiveIDs[q.ID] = struct{}{}
		}
	}

	type perAttempt struct {
		valid          bool
		earned         float64
		max            float64
		objectiveTotal int
		correctTotal   int
	}
	type kpAgg struct {
		questionCount  int
		objectiveCount int
		attempts       map[uint]*perAttempt
	}
	aggByKP := make(map[string]*kpAgg, len(metaByKP))
	for kp, meta := range metaByKP {
		aggByKP[kp] = &kpAgg{
			questionCount:  len(meta.questionIDs),
			objectiveCount: len(meta.objectiveIDs),
			attempts:       map[uint]*perAttempt{},
		}
	}

	for _, a := range answers {
		it, ok := eqByID[a.ExamQuestionID]
		if !ok {
			continue
		}
		q, ok := questions[a.QuestionID]
		if !ok || q.KnowledgePoint == "" {
			continue
		}
		agg := aggByKP[q.KnowledgePoint]
		entry, exists := agg.attempts[a.AttemptID]
		if !exists {
			entry = &perAttempt{}
			agg.attempts[a.AttemptID] = entry
		}
		entry.earned += a.Score
		entry.max += it.Score
		if a.AnswerText != "" {
			entry.valid = true
		}
		if ObjectiveQuestionTypes()[q.Type] {
			entry.objectiveTotal++
			if a.IsCorrect != nil && *a.IsCorrect {
				entry.correctTotal++
			}
		}
	}

	result := make([]dto.KnowledgePointStat, 0, len(aggByKP))
	for kp, agg := range aggByKP {
		participants := 0
		earnedSum := 0.0
		maxSum := 0.0
		objectiveItems := 0
		correctItems := 0
		for _, entry := range agg.attempts {
			if !entry.valid {
				continue
			}
			participants++
			earnedSum += entry.earned
			maxSum += entry.max
			objectiveItems += entry.objectiveTotal
			correctItems += entry.correctTotal
		}
		if participants == 0 {
			continue
		}
		scoreRate := 0.0
		if maxSum > 0 {
			scoreRate = earnedSum / maxSum * 100
		}
		stat := dto.KnowledgePointStat{
			KnowledgePoint:   kp,
			QuestionCount:    agg.questionCount,
			ParticipantCount: participants,
			AverageScoreRate: round2(scoreRate),
			Weak:             scoreRate < weakScoreRate,
		}
		if agg.objectiveCount > 0 {
			accuracy := 0.0
			if objectiveItems > 0 {
				accuracy = float64(correctItems) / float64(objectiveItems) * 100
			}
			stat.ObjectiveAccuracy = ptrRound2(accuracy)
		}
		result = append(result, stat)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].AverageScoreRate != result[j].AverageScoreRate {
			return result[i].AverageScoreRate < result[j].AverageScoreRate
		}
		return result[i].KnowledgePoint < result[j].KnowledgePoint
	})
	return result
}

func ptrRound2(v float64) *float64 {
	rounded := round2(v)
	return &rounded
}

func buildScoreBuckets(attempts []model.ExamAttempt, totalScore float64) []dto.ScoreBucket {
	labels := []string{"0-59", "60-69", "70-79", "80-89", "90-100"}
	buckets := make([]dto.ScoreBucket, len(labels))
	for i, label := range labels {
		buckets[i] = dto.ScoreBucket{Label: label, Count: 0}
	}
	for _, a := range attempts {
		percent := 100.0
		if totalScore > 0 {
			percent = a.TotalScore / totalScore * 100
		}
		switch {
		case percent < 60:
			buckets[0].Count++
		case percent < 70:
			buckets[1].Count++
		case percent < 80:
			buckets[2].Count++
		case percent < 90:
			buckets[3].Count++
		default:
			buckets[4].Count++
		}
	}
	return buckets
}
