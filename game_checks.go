package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

const checkRule = "d20-graded-v1"

var checkGrades = []string{"critical_success", "great_success", "success", "minor_failure", "failure", "critical_failure"}
var checkLabels = map[string]string{
	"critical_success": "大成功", "great_success": "出色成功", "success": "普通成功",
	"minor_failure": "轻度失败", "failure": "明显失败", "critical_failure": "大失败",
}

type checkEffect struct {
	Description string  `json:"description"`
	HPDelta     int     `json:"hp_delta"`
	Conditions  *string `json:"conditions,omitempty"`
	Inventory   *string `json:"inventory,omitempty"`
	Resources   *string `json:"resources,omitempty"`
}
type checkSnapshot struct {
	HP         int    `json:"hp"`
	MaxHP      int    `json:"max_hp"`
	Conditions string `json:"conditions"`
	Inventory  string `json:"inventory"`
	Resources  string `json:"resources"`
}
type abilityCheck struct {
	ID               int64                  `json:"id"`
	CampaignID       int64                  `json:"campaign_id"`
	EventID          int64                  `json:"event_id"`
	CharacterID      int64                  `json:"character_id"`
	UserID           int64                  `json:"user_id"`
	CharacterName    string                 `json:"character_name"`
	TurnID           int64                  `json:"turn_id"`
	Rule             string                 `json:"rule"`
	Kind             string                 `json:"kind"`
	Ability          string                 `json:"ability"`
	Skill            string                 `json:"skill"`
	Reason           string                 `json:"reason"`
	AbilityScore     int                    `json:"ability_score"`
	AbilityModifier  int                    `json:"ability_modifier"`
	Bonus            int                    `json:"bonus"`
	BonusReason      string                 `json:"bonus_reason"`
	Modifier         int                    `json:"modifier"`
	DC               int                    `json:"dc"`
	Cost             checkEffect            `json:"cost"`
	Outcomes         map[string]checkEffect `json:"outcomes"`
	Offer            *checkEffect           `json:"offer,omitempty"`
	Status           string                 `json:"status"`
	Natural          int                    `json:"natural"`
	Total            int                    `json:"total"`
	Margin           int                    `json:"margin"`
	Grade            string                 `json:"grade"`
	Accepted         *bool                  `json:"accepted,omitempty"`
	Resolution       string                 `json:"resolution"`
	Before           checkSnapshot          `json:"before"`
	After            checkSnapshot          `json:"after"`
	CharacterVersion int64                  `json:"character_version"`
	CreatedAt        int64                  `json:"created_at"`
	RolledAt         int64                  `json:"rolled_at"`
	ResolvedAt       int64                  `json:"resolved_at"`
}
type checkPrepareInput struct {
	gameOp
	EventID     int64                  `json:"event_id"`
	CharacterID int64                  `json:"character_id"`
	TurnID      int64                  `json:"turn_id"`
	Lease       string                 `json:"lease"`
	Kind        string                 `json:"kind"`
	Ability     string                 `json:"ability"`
	Skill       string                 `json:"skill"`
	Reason      string                 `json:"reason"`
	Bonus       int                    `json:"bonus"`
	BonusReason string                 `json:"bonus_reason"`
	DC          int                    `json:"dc"`
	Cost        checkEffect            `json:"cost"`
	Outcomes    map[string]checkEffect `json:"outcomes"`
	Offer       *checkEffect           `json:"offer"`
}
type checkRollInput struct {
	gameOp
	TurnID int64  `json:"turn_id"`
	Lease  string `json:"lease"`
}

func gradedCheck(natural, total, dc int) string {
	switch {
	case natural == 20:
		return "critical_success"
	case natural == 1:
		return "critical_failure"
	case total-dc >= 5:
		return "great_success"
	case total >= dc:
		return "success"
	case total-dc >= -4:
		return "minor_failure"
	default:
		return "failure"
	}
}
func validateCheckEffect(e checkEffect) error {
	if !textOK(e.Description, 1, 1000) || e.HPDelta < -9999 || e.HPDelta > 9999 {
		return gameFail(400, "每项后果需填写 1–1000 字说明，生命值变化范围为 ±9999")
	}
	for _, text := range []*string{e.Conditions, e.Inventory, e.Resources} {
		if text != nil && !textOK(*text, 0, 6000) {
			return gameFail(400, "状态、物品和资源各最多 6000 字")
		}
	}
	return nil
}
func checkEffectResult(s checkSnapshot, effect checkEffect) (checkSnapshot, error) {
	s.HP += effect.HPDelta
	if s.HP < 0 || s.HP > s.MaxHP {
		return s, gameFail(400, "预定后果使生命值超出 0～最大生命值，请调整后果")
	}
	if effect.Conditions != nil {
		s.Conditions = *effect.Conditions
	}
	if effect.Inventory != nil {
		s.Inventory = *effect.Inventory
	}
	if effect.Resources != nil {
		s.Resources = *effect.Resources
	}
	return s, nil
}
func pendingCharacterCheck(tx *sql.Tx, cid, characterID int64) error {
	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM campaign_checks WHERE campaign_id=? AND character_id=? AND status IN ('prepared','awaiting_choice')", cid, characterID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return gameFail(409, "该角色有已锁定检定或待确认代价，请先完成检定")
	}
	return nil
}
func loadAbilityCheck(row scanner) (abilityCheck, error) {
	var raw string
	var check abilityCheck
	if err := row.Scan(&raw); err != nil {
		return check, err
	}
	err := json.Unmarshal([]byte(raw), &check)
	return check, err
}
func saveAbilityCheck(tx *sql.Tx, check abilityCheck) error {
	_, err := tx.Exec("UPDATE campaign_checks SET status=?,document=? WHERE id=? AND campaign_id=?", check.Status, gameJSON(check), check.ID, check.CampaignID)
	return err
}
func (a *app) gameChecks(r *http.Request, g gameActor) (any, error) {
	c, err := a.gameAccess(pathID(r, "id"), g)
	if err != nil {
		return nil, err
	}
	if id := pathID(r, "check"); id > 0 {
		check, err := loadAbilityCheck(a.store.QueryRow("SELECT document FROM campaign_checks WHERE campaign_id=? AND id=?", c.ID, id))
		if err == sql.ErrNoRows {
			return nil, gameFail(404, "检定不存在")
		}
		return check, err
	}
	return a.publicChecks(c.ID)
}
func (a *app) publicChecks(cid int64) ([]abilityCheck, error) {
	rows, err := a.store.Query("SELECT document FROM campaign_checks WHERE campaign_id=? ORDER BY CASE WHEN status IN ('prepared','awaiting_choice') THEN 0 ELSE 1 END,id DESC LIMIT 80", cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []abilityCheck{}
	for rows.Next() {
		check, err := loadAbilityCheck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, check)
	}
	return out, rows.Err()
}
func (a *app) gamePrepareCheck(r *http.Request, g gameActor) (any, error) {
	var in checkPrepareInput
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.prepareCheck(pathID(r, "id"), g, in)
}
func (a *app) prepareCheck(cid int64, g gameActor, in checkPrepareInput) (any, error) {
	fingerprint := in
	fingerprint.Lease = ""
	return a.gameMutation(cid, g, in.gameOp, "check-prepare", fingerprint, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.host(c) {
			return nil, gameFail(403, "检定条件由房主或主持智能体确定")
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "战役已暂停")
		}
		if in.TurnID != 0 {
			if err := checkGameLease(tx, c, g, in.TurnID, in.Lease); err != nil {
				return nil, err
			}
			var event int64
			if err := tx.QueryRow("SELECT event_id FROM campaign_turns WHERE id=?", in.TurnID).Scan(&event); err != nil {
				return nil, err
			}
			if event != in.EventID {
				return nil, gameFail(400, "只能为当前回合的玩家行动创建检定")
			}
		} else if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		if in.Kind != "ability" && in.Kind != "skill" {
			return nil, gameFail(400, "分级房规仅用于属性或技能检定")
		}
		switch in.Ability {
		case "str", "dex", "con", "int", "wis", "cha":
		default:
			return nil, gameFail(400, "检定属性无效")
		}
		if !textOK(in.Reason, 1, 200) || !textOK(in.Skill, 0, 60) || (in.Kind == "skill" && !textOK(in.Skill, 1, 60)) ||
			in.DC < 1 || in.DC > 40 || in.Bonus < -20 || in.Bonus > 20 || !textOK(in.BonusReason, 0, 200) || (in.Bonus != 0 && !textOK(in.BonusReason, 1, 200)) {
			return nil, gameFail(400, "填写检定目的、技能及修正依据；DC 为 1–40，额外修正为 ±20")
		}
		if len(in.Outcomes) != len(checkGrades) {
			return nil, gameFail(400, "投骰前须确定全部六种结果的后果")
		}
		effects := []checkEffect{in.Cost}
		for _, grade := range checkGrades {
			effect, ok := in.Outcomes[grade]
			if !ok {
				return nil, gameFail(400, "缺少结果等级："+grade)
			}
			effects = append(effects, effect)
		}
		if in.Offer != nil {
			effects = append(effects, *in.Offer)
		}
		for _, effect := range effects {
			if err := validateCheckEffect(effect); err != nil {
				return nil, err
			}
		}
		check := abilityCheck{CampaignID: cid, EventID: in.EventID, CharacterID: in.CharacterID, TurnID: in.TurnID,
			Rule: checkRule, Kind: in.Kind, Ability: in.Ability, Skill: in.Skill, Reason: in.Reason, DC: in.DC,
			Bonus: in.Bonus, BonusReason: in.BonusReason, Cost: in.Cost, Outcomes: in.Outcomes, Offer: in.Offer,
			Status: "prepared", CreatedAt: time.Now().UnixMilli()}
		var abilities string
		err := tx.QueryRow("SELECT user_id,name,abilities,hp,max_hp,conditions,inventory,resources,version FROM characters WHERE campaign_id=? AND id=?", cid, in.CharacterID).
			Scan(&check.UserID, &check.CharacterName, &abilities, &check.Before.HP, &check.Before.MaxHP, &check.Before.Conditions, &check.Before.Inventory, &check.Before.Resources, &check.CharacterVersion)
		if err == sql.ErrNoRows {
			return nil, gameFail(400, "角色不属于此战役")
		}
		if err != nil {
			return nil, err
		}
		var actor int64
		var kind string
		if tx.QueryRow("SELECT actor_id,kind FROM campaign_events WHERE campaign_id=? AND id=? AND visibility='public'", cid, in.EventID).Scan(&actor, &kind) != nil || kind != "action" || actor != check.UserID {
			return nil, gameFail(400, "必须绑定该角色玩家提交的公开行动")
		}
		var existing int
		if err = tx.QueryRow("SELECT COUNT(*) FROM campaign_checks WHERE campaign_id=? AND event_id=?", cid, in.EventID).Scan(&existing); err != nil {
			return nil, err
		}
		if existing != 0 {
			return nil, gameFail(409, "此行动已有锁定检定，请读取原检定，不能另投一次")
		}
		if err = pendingCharacterCheck(tx, cid, in.CharacterID); err != nil {
			return nil, err
		}
		var scores map[string]int
		if err = json.Unmarshal([]byte(abilities), &scores); err != nil {
			return nil, err
		}
		check.AbilityScore = scores[in.Ability]
		check.AbilityModifier = int(math.Floor(float64(check.AbilityScore-10) / 2))
		check.Modifier = check.AbilityModifier + in.Bonus
		check.After = check.Before
		costState, err := checkEffectResult(check.Before, in.Cost)
		if err != nil {
			return nil, err
		}
		for _, effect := range effects[1:] {
			if _, err = checkEffectResult(costState, effect); err != nil {
				return nil, err
			}
		}
		res, err := tx.Exec("INSERT INTO campaign_checks(campaign_id,event_id,character_id,turn_id,status,document) VALUES(?,?,?,?,?,'{}')", cid, in.EventID, in.CharacterID, in.TurnID, check.Status)
		if err != nil {
			return nil, err
		}
		check.ID, _ = res.LastInsertId()
		if err = saveAbilityCheck(tx, check); err != nil {
			return nil, err
		}
		_, err = gameEvent(tx, c, c.BotID, "check", "public", 0, fmt.Sprintf("%s：%s · DC %d · 修正 %+d；检定条件已锁定（属性／技能分级房规）", check.CharacterName, check.Reason, check.DC, check.Modifier), map[string]any{"check": check})
		return check, err
	})
}
func updateCheckCharacter(tx *sql.Tx, c campaign, check *abilityCheck) error {
	res, err := tx.Exec("UPDATE characters SET hp=?,conditions=?,inventory=?,resources=?,version=version+1 WHERE id=? AND campaign_id=? AND version=?",
		check.After.HP, check.After.Conditions, check.After.Inventory, check.After.Resources, check.CharacterID, c.ID, check.CharacterVersion)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return gameFail(409, "角色资料已变化，检定结果未应用")
	}
	check.CharacterVersion++
	// A claimed turn keeps its base version until completion. Other state writers
	// are excluded by its lease; completion cannot apply this character twice.
	if c.ActiveTurn == 0 {
		return bumpGame(tx, c.ID)
	}
	return nil
}
func (a *app) gameRollCheck(r *http.Request, g gameActor) (any, error) {
	var in checkRollInput
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.rollCheck(pathID(r, "id"), pathID(r, "check"), g, in)
}
func (a *app) rollCheck(cid, id int64, g gameActor, in checkRollInput) (any, error) {
	fingerprint := in
	fingerprint.Lease = ""
	return a.gameMutation(cid, g, in.gameOp, fmt.Sprintf("check-roll:%d", id), fingerprint, func(tx *sql.Tx, c campaign) (any, error) {
		check, err := loadAbilityCheck(tx.QueryRow("SELECT document FROM campaign_checks WHERE campaign_id=? AND id=?", cid, id))
		if err == sql.ErrNoRows {
			return nil, gameFail(404, "检定不存在")
		}
		if err != nil {
			return nil, err
		}
		if !g.host(c) && g.ID != check.UserID {
			return nil, gameFail(403, "只能投自己角色的检定")
		}
		if check.Status != "prepared" {
			return check, nil
		}
		if c.Phase == "paused" {
			return nil, gameFail(409, "战役已暂停")
		}
		if check.TurnID != 0 {
			if in.TurnID != check.TurnID {
				return nil, gameFail(409, "请使用检定所属回合的租约")
			}
			if err = checkGameLease(tx, c, g, in.TurnID, in.Lease); err != nil {
				return nil, err
			}
		} else if c.ActiveTurn != 0 {
			return nil, gameFail(409, "请等待当前主持回合结束")
		}
		roll, err := rollDice("1d20")
		if err != nil {
			return nil, err
		}
		check.Natural = roll.Dice[0]
		check.Total = check.Natural + check.Modifier
		check.Margin = check.Total - check.DC
		check.Grade = gradedCheck(check.Natural, check.Total, check.DC)
		check.RolledAt = time.Now().UnixMilli()
		check.After, err = checkEffectResult(check.Before, check.Cost)
		if err != nil {
			return nil, err
		}
		check.Status = "resolved"
		check.ResolvedAt = check.RolledAt
		check.Resolution = check.Outcomes[check.Grade].Description
		if check.Grade == "minor_failure" && check.Offer != nil {
			check.Status = "awaiting_choice"
			check.ResolvedAt = 0
			check.Resolution = "等待角色所属玩家选择是否接受代价"
		} else {
			check.After, err = checkEffectResult(check.After, check.Outcomes[check.Grade])
			if err != nil {
				return nil, err
			}
		}
		if err = updateCheckCharacter(tx, c, &check); err != nil {
			return nil, err
		}
		if err = saveAbilityCheck(tx, check); err != nil {
			return nil, err
		}
		_, err = gameEvent(tx, c, c.BotID, "check", "public", 0, fmt.Sprintf("%s：d20=%d，修正 %+d，总点数 %d / DC %d → %s。%s", check.CharacterName, check.Natural, check.Modifier, check.Total, check.DC, checkLabels[check.Grade], check.Resolution), map[string]any{"check": check})
		return check, err
	})
}
func (a *app) gameChooseCheck(r *http.Request, g gameActor) (any, error) {
	var in struct {
		gameOp
		Accept *bool `json:"accept"`
	}
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	if in.Accept == nil {
		return nil, gameFail(400, "请选择接受或拒绝代价")
	}
	out, err := a.gameMutation(pathID(r, "id"), g, in.gameOp, fmt.Sprintf("check-choice:%d", pathID(r, "check")), in, func(tx *sql.Tx, c campaign) (any, error) {
		check, err := loadAbilityCheck(tx.QueryRow("SELECT document FROM campaign_checks WHERE campaign_id=? AND id=?", c.ID, pathID(r, "check")))
		if err == sql.ErrNoRows {
			return nil, gameFail(404, "检定不存在")
		}
		if err != nil {
			return nil, err
		}
		if g.AgentID != 0 || g.ID != check.UserID {
			return nil, gameFail(403, "只有角色所属玩家能接受或拒绝代价")
		}
		if check.Accepted != nil {
			if *check.Accepted == *in.Accept {
				return check, nil
			}
			return nil, gameFail(409, "该检定已作出选择")
		}
		if check.Status != "awaiting_choice" || check.Offer == nil {
			return nil, gameFail(409, "该检定没有待确认代价")
		}
		if c.ActiveTurn != 0 || c.Phase == "paused" {
			return nil, gameFail(409, "请等待主持完成当前回合并继续战役后选择")
		}
		effect := check.Outcomes["minor_failure"]
		if *in.Accept {
			effect = *check.Offer
		}
		check.After, err = checkEffectResult(check.After, effect)
		if err != nil {
			return nil, err
		}
		check.Accepted = in.Accept
		check.Resolution = effect.Description
		check.Status = "resolved"
		check.ResolvedAt = time.Now().UnixMilli()
		if err = updateCheckCharacter(tx, c, &check); err != nil {
			return nil, err
		}
		if err = saveAbilityCheck(tx, check); err != nil {
			return nil, err
		}
		id, err := gameEvent(tx, c, g.ID, "check", "public", 0, fmt.Sprintf("%s 的玩家已选择：%s", check.CharacterName, check.Resolution), map[string]any{"check": check})
		if err != nil {
			return nil, err
		}
		if c.AutoRespond && c.Mode != "manual" {
			// Resolving a blocked choice must remain possible even when player
			// actions filled the queue. This is a continuation, not a new action.
			if _, err = tx.Exec("INSERT INTO campaign_turns(campaign_id,event_id,kind) VALUES(?,?,'action')", c.ID, id); err != nil {
				return nil, err
			}
		}
		return check, nil
	})
	if err == nil {
		a.kickGame(pathID(r, "id"))
	}
	return out, err
}

func (a *app) gameCancelCheck(r *http.Request, g gameActor) (any, error) {
	var in gameOp
	if err := gameBody(r, &in); err != nil {
		return nil, err
	}
	return a.gameMutation(pathID(r, "id"), g, in, fmt.Sprintf("check-cancel:%d", pathID(r, "check")), in, func(tx *sql.Tx, c campaign) (any, error) {
		if !g.owner(c) {
			return nil, gameFail(403, "只有房主能取消尚未投骰的检定")
		}
		if err := gameVersion(c, in.Version); err != nil {
			return nil, err
		}
		check, err := loadAbilityCheck(tx.QueryRow("SELECT document FROM campaign_checks WHERE campaign_id=? AND id=?", c.ID, pathID(r, "check")))
		if err == sql.ErrNoRows {
			return nil, gameFail(404, "检定不存在")
		}
		if err != nil {
			return nil, err
		}
		if check.Status == "cancelled" {
			return check, nil
		}
		if check.Status != "prepared" {
			return nil, gameFail(409, "已投骰的检定不能取消或重掷")
		}
		check.Status = "cancelled"
		check.Resolution = "房主取消，未投骰、未扣资源；重新尝试需玩家提交新行动"
		if err = saveAbilityCheck(tx, check); err != nil {
			return nil, err
		}
		_, err = gameEvent(tx, c, c.OwnerID, "check", "public", 0, check.Resolution, map[string]any{"check": check})
		return check, err
	})
}

// Event/turn/lease are supplied by the server, never invented by a model.
func checkTools() []any {
	text := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer"}
	effect := map[string]any{"type": "object", "properties": map[string]any{
		"description": text, "hp_delta": integer, "conditions": text, "inventory": text, "resources": text,
	}, "required": []string{"description", "hp_delta"}, "additionalProperties": false}
	outcomes := map[string]any{}
	for _, grade := range checkGrades {
		outcomes[grade] = effect
	}
	return []any{
		gameTool("prepare_check", "属性/技能检定房规：投骰前锁定角色、属性、DC、消耗、六档后果；不可完成不可能事件或凭空即死。生命值为增减值，其余资源为完整替换；轻度失败可提供由玩家选择的代价成功 offer。属性修正由服务器读取角色计算。", map[string]any{
			"character_id": integer, "kind": map[string]any{"type": "string", "enum": []string{"ability", "skill"}},
			"ability": map[string]any{"type": "string", "enum": []string{"str", "dex", "con", "int", "wis", "cha"}},
			"skill":   text, "reason": text, "bonus": integer, "bonus_reason": text, "dc": integer, "cost": effect,
			"outcomes": map[string]any{"type": "object", "properties": outcomes, "required": checkGrades, "additionalProperties": false}, "offer": effect,
		}, []string{"character_id", "kind", "ability", "reason", "dc", "cost", "outcomes"}),
		gameTool("roll_check", "对已锁定检定投骰，软件计算等级并应用后果。返回 awaiting_choice 时等待玩家选择，不能替玩家接受；finish_turn 不再修改此角色资源。", map[string]any{"check_id": integer}, []string{"check_id"}),
	}
}
