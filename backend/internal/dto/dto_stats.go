package dto

import "time"

// RankItem is one student in a score ranking.
type RankItem struct {
	Rank            int        `json:"rank"`
	StudentName     string     `json:"student_name"`
	StudentUsername string     `json:"student_username"`
	TotalScore      float64    `json:"total_score"`
	SubmittedAt     *time.Time `json:"submitted_at"`
}

// ScoreBucket is a histogram bucket.
type ScoreBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// KnowledgePointStat aggregates mastery of one knowledge point in an exam.
type KnowledgePointStat struct {
	KnowledgePoint    string   `json:"knowledge_point"`
	QuestionCount     int      `json:"question_count"`
	ParticipantCount  int      `json:"participant_count"`
	AvgScoreRate      float64  `json:"avg_score_rate"`
	ObjectiveAccuracy *float64 `json:"objective_accuracy"`
	Weak              bool     `json:"weak"`
}

// ExamStatResponse is teacher-facing exam statistics.
type ExamStatResponse struct {
	ExamID            uint                 `json:"exam_id"`
	ExamTitle         string               `json:"exam_title"`
	ParticipantCount  int                  `json:"participant_count"`
	AverageScore      float64              `json:"average_score"`
	HighestScore      float64              `json:"highest_score"`
	LowestScore       float64              `json:"lowest_score"`
	PassCount         int                  `json:"pass_count"`
	ScoreDistribution []ScoreBucket        `json:"score_distribution"`
	Ranking           []RankItem           `json:"ranking"`
	KnowledgePoints   []KnowledgePointStat `json:"knowledge_points"`
}

// OverviewResponse is a compact dashboard summary.
type OverviewResponse struct {
	UserCount     int64 `json:"user_count"`
	QuestionCount int64 `json:"question_count"`
	ExamCount     int64 `json:"exam_count"`
	AttemptCount  int64 `json:"attempt_count"`
}
