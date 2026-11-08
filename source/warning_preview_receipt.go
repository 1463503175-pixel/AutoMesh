package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// A warning proposal is a durable, separately reviewed product of a completed
// ACT job. It is NOT the same as the last general quality-report result.
// This record is authored only after a verified successful preview result.
type WarningPreviewReceipt struct {
	JobID       string `json:"job_id"`
	PlanID      string `json:"plan_id"`
	Project     string `json:"project"`
	Fingerprint string `json:"geometry_fingerprint"`
	CreatedAt   string `json:"created_at"`
}

func (w Workspace) warningPreviewReceiptPath() string { return w.path("warning_preview_receipt.json") }

func (w Workspace) RecordWarningPreview(result Result, current *Snapshot) error {
	if current == nil || result.Status != "ok" || result.Phase != "warning_refine_preview" || result.JobID == "" {
		return errors.New("只能记录当前工程已完成的警告区预览任务")
	}
	var summary map[string]any
	if err := json.Unmarshal(result.Summary, &summary); err != nil {
		return fmt.Errorf("警告区预览响应不可解析: %w", err)
	}
	planID, _ := summary["plan_id"].(string)
	if planID == "" {
		return errors.New("Meshing 未返回警告区方案 ID")
	}
	b, readErr := os.ReadFile(w.path("warning_refinement_plan.json"))
	if readErr != nil {
		return readErr
	}
	var plan WarningPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return err
	}
	if err := validateWarningDiagnostic(plan, current, planID); err != nil {
		return fmt.Errorf("预览结果与诊断文件不一致: %w", err)
	}
	return writeJSON(w.warningPreviewReceiptPath(), WarningPreviewReceipt{
		JobID: result.JobID, PlanID: planID, Project: current.Project,
		Fingerprint: current.Fingerprint, CreatedAt: time.Now().Format(time.RFC3339),
	})
}

func (w Workspace) ReadReviewedWarningDiagnostic(current *Snapshot) (WarningPlan, error) {
	if current == nil {
		return WarningPlan{}, errors.New("没有当前工程快照")
	}
	proof, err := readJSON[WarningPreviewReceipt](w.warningPreviewReceiptPath())
	if err != nil || proof.PlanID == "" || proof.Project != current.Project || proof.Fingerprint != current.Fingerprint {
		return WarningPlan{}, errors.New("没有匹配当前工程的已完成警告区诊断")
	}
	b, err := os.ReadFile(w.path("warning_refinement_plan.json"))
	if err != nil {
		return WarningPlan{}, err
	}
	var plan WarningPlan
	if err = json.Unmarshal(b, &plan); err != nil {
		return WarningPlan{}, err
	}
	if err = validateWarningDiagnostic(plan, current, proof.PlanID); err != nil {
		return WarningPlan{}, err
	}
	return plan, nil
}
func (w Workspace) ReadReviewedWarningPlan(current *Snapshot) (WarningPlan, error) {
	if current == nil {
		return WarningPlan{}, errors.New("当前没有已验证的 Meshing 几何快照，请先只读扫描")
	}
	proof, err := readJSON[WarningPreviewReceipt](w.warningPreviewReceiptPath())
	if err != nil || proof.PlanID == "" || proof.JobID == "" {
		return WarningPlan{}, errors.New("未找到已成功完成的警告区方案；请先生成并确认预览结束")
	}
	if proof.Project != current.Project || proof.Fingerprint != current.Fingerprint {
		return WarningPlan{}, errors.New("已保存的警告区方案属于不同工程或几何，拒绝发布")
	}
	return w.loadWarningPlan(current, proof.PlanID)
}

func (w Workspace) loadWarningPlan(current *Snapshot, planID string) (WarningPlan, error) {
	b, err := os.ReadFile(w.path("warning_refinement_plan.json"))
	if err != nil {
		return WarningPlan{}, err
	}
	var plan WarningPlan
	if err = json.Unmarshal(b, &plan); err != nil {
		return WarningPlan{}, err
	}
	if err = validateWarningPlan(plan, current, planID); err != nil {
		return WarningPlan{}, err
	}
	return plan, nil
}

func (w Workspace) InvalidateWarningPreview() error {
	err := os.Remove(w.warningPreviewReceiptPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
