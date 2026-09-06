package compiler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jin-take/SplitAgents/internal/domain"
	"github.com/jin-take/SplitAgents/internal/openai"
)

type Compiler struct{ AI *openai.Client }
func New(ai *openai.Client)*Compiler{return &Compiler{AI:ai}}

func (c *Compiler) Plan(ctx context.Context, prompt string, wantTitle bool) domain.Plan {
	p:=heuristic(prompt)
	if !wantTitle && len([]rune(prompt))<140 && !looksComplex(prompt) { return p }
	inst:="Act only as a low-cost request planner. Do not answer the task. Return compact JSON only with title, task_summary, complexity, model, reasoning, context_keywords, subagents, recent_messages. title must be 3-7 English words. model must be one of gpt-5.6-luna, gpt-5.6-terra, gpt-5.6-sol. reasoning must be none, low, medium, or high. subagents is 0-3. Optimize total token cost and prefer one agent."
	text,_,err:=c.AI.Complete(ctx,"gpt-5.6-luna","low",inst,prompt,450); if err!=nil{return p}
	text=strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(text),"```json"),"```")); var x domain.Plan; if json.Unmarshal([]byte(text),&x)!=nil{return p}; x.PlannerUsed=true; if !wantTitle{x.Title=""}; normalize(&x); return x
}

func heuristic(prompt string) domain.Plan { p:=domain.Plan{TaskSummary:prompt,Complexity:"normal",Model:"gpt-5.6-terra",Reasoning:"low",Subagents:0,RecentMessages:6}; if len([]rune(prompt))<120 { p.Complexity="simple";p.Model="gpt-5.6-luna";p.Reasoning="none";p.RecentMessages=4 }; if looksComplex(prompt){p.Complexity="complex";p.Model="gpt-5.6-sol";p.Reasoning="medium";p.RecentMessages=8}; return p }
func looksComplex(s string)bool{l:=strings.ToLower(s); for _,k:=range []string{"architecture","設計","調査","research","review","複数","比較","実装","debug","原因","security"}{if strings.Contains(l,k){return true}}; return len([]rune(s))>500}
func normalize(p *domain.Plan){if p.Model==""{p.Model="gpt-5.6-terra"}; if p.Reasoning==""{p.Reasoning="low"}; if p.RecentMessages<2{p.RecentMessages=2};if p.RecentMessages>10{p.RecentMessages=10};if p.Subagents<0{p.Subagents=0};if p.Subagents>3{p.Subagents=3}}
