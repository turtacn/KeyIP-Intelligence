package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// AIHandler 提供 AI 驱动的专利分析端点（使用 DeepSeek/OpenAI 兼容后端）
type AIHandler struct {
	backend common.ModelBackend
	logger  logging.Logger
}

func NewAIHandler(backend common.ModelBackend, logger logging.Logger) *AIHandler {
	return &AIHandler{backend: backend, logger: logger}
}

func (h *AIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ai/analyze-patent", h.AnalyzePatent)
	mux.HandleFunc("POST /api/v1/ai/chat", h.Chat)
	mux.HandleFunc("GET /api/v1/ai/health", h.Health)
}

type AnalyzePatentRequest struct {
	PatentTitle    string `json:"patent_title"`
	PatentAbstract string `json:"patent_abstract"`
	MoleculeName   string `json:"molecule_name,omitempty"`
}

func (h *AIHandler) AnalyzePatent(w http.ResponseWriter, r *http.Request) {
	var req AnalyzePatentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid JSON"))
		return
	}
	if req.PatentTitle == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent_title", "required"))
		return
	}

	prompt := "你是一位OLED材料专利分析专家。请分析以下专利:"
	if req.MoleculeName != "" {
		prompt += " 涉及分子: " + req.MoleculeName + "。"
	}
	prompt += " 标题: " + req.PatentTitle
	if req.PatentAbstract != "" {
		prompt += " 摘要: " + req.PatentAbstract
	}
	prompt += " 请评估: 1)技术价值 2)保护范围 3)潜在侵权风险 4)商业化前景。每个维度给出1-10分评分。用中文回答，控制在300字以内。"

	input, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 800, "temperature": 0.5,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := h.backend.Predict(ctx, &common.PredictRequest{
		ModelName: "deepseek-chat",
		InputData: input,
	})
	if err != nil {
		h.logger.Error("AI predict failed", logging.Err(err))
		writeError(w, http.StatusInternalServerError, errors.NewInternal("AI analysis failed"))
		return
	}

	content := string(resp.Outputs["content"])
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"analysis": content,
		"tokens":   resp.Metadata,
	})
}

func (h *AIHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid JSON"))
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("message", "required"))
		return
	}

	input, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": "你是KeyIP智能知识产权助手，专门帮助用户分析OLED材料相关专利。请用中文回答，保持专业准确。"},
			{"role": "user", "content": req.Message},
		},
		"max_tokens": 1000, "temperature": 0.7,
	})

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := h.backend.Predict(ctx, &common.PredictRequest{
		ModelName: "deepseek-chat",
		InputData: input,
	})
	if err != nil {
		h.logger.Error("AI chat failed", logging.Err(err))
		writeError(w, http.StatusInternalServerError, errors.NewInternal("AI chat failed"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reply":  string(resp.Outputs["content"]),
		"tokens": resp.Metadata,
	})
}

func (h *AIHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.backend.Healthy(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
