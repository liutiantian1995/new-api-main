package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

func GetGroupEnabledModels(group string) []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where(commonGroupCol+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func getPriority(group string, model string, retry int) (int, error) {

	var priorities []int
	err := DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true).
		Order("priority DESC").              // 按优先级降序排序
		Pluck("priority", &priorities).Error // Pluck用于将查询的结果直接扫描到一个切片中

	if err != nil {
		// 处理错误
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

func getChannelQuery(group string, model string, retry int) (*gorm.DB, error) {
	maxPrioritySubQuery := DB.Model(&Ability{}).Select("MAX(priority)").Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, model, true)
	channelQuery := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = (?)", group, model, true, maxPrioritySubQuery)
	if retry != 0 {
		priority, err := getPriority(group, model, retry)
		if err != nil {
			return nil, err
		} else {
			channelQuery = DB.Where(commonGroupCol+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priority)
		}
	}

	return channelQuery, nil
}

// GetChannel selects a channel directly from the database (used when memory cache is disabled).
// Since v1.0.0-rc.15-batch4, the DB path also honors token-aware routing:
//   - estTokens > 0 时先按 max_tokens 软过滤候选渠道
//   - 全部被过滤则回退到原始全集合，fallback 返回 true
//   - 再按 effective_priority (base + Σ tier boost) 分 tier，retry 索引选择 tier
//   - 同 tier 内沿用 weight 加权随机
//
// 与内存缓存路径 (GetRandomSatisfiedChannel) 行为对齐，避免 DB 模式下 token 路由失效。
func GetChannel(group string, model string, retry int, requestPath string, estTokens int) (*Channel, bool, error) {
	var abilities []Ability

	var err error = nil
	channelQuery, err := getChannelQuery(group, model, retry)
	if err != nil {
		return nil, false, err
	}
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) || common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	} else {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	}
	if err != nil {
		return nil, false, err
	}
	abilities = filterAbilitiesByRequestPath(abilities, requestPath)
	if len(abilities) == 0 {
		return nil, false, nil
	}

	// 收集候选 channel id 并按 max_tokens 软过滤
	candidateIds := make([]int, 0, len(abilities))
	for _, a := range abilities {
		candidateIds = append(candidateIds, a.ChannelId)
	}

	// 加载候选 Channel 行（含 MaxTokens / TokenTiers 字段，供 effective_priority 计算）
	channelsById, err := loadChannelsByIds(candidateIds)
	if err != nil {
		return nil, false, err
	}

	filteredIds, fallback := filterChannelsByMaxTokensFromMap(candidateIds, channelsById, estTokens)
	if fallback {
		// 全部被 max_tokens 软过滤 → 回退到原始全集合
		filteredIds = candidateIds
	}

	// effective_priority 分 tier
	uniqueEffectivePriorities := make(map[int64]bool)
	for _, id := range filteredIds {
		ch, ok := channelsById[id]
		if !ok {
			return nil, false, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", id)
		}
		uniqueEffectivePriorities[computeEffectivePriority(ch, estTokens)] = true
	}
	sortedUniquePriorities := make([]int64, 0, len(uniqueEffectivePriorities))
	for p := range uniqueEffectivePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, p)
	}
	sort.Slice(sortedUniquePriorities, func(i, j int) bool { return sortedUniquePriorities[i] > sortedUniquePriorities[j] })

	retryIdx := retry
	if retryIdx >= len(sortedUniquePriorities) {
		retryIdx = len(sortedUniquePriorities) - 1
	}
	targetPriority := sortedUniquePriorities[retryIdx]

	// 同 effective tier 内 weight 加权随机
	// 对齐内存缓存路径 (GetRandomSatisfiedChannel) 的 smoothing 逻辑：
	//   - weight 全 0 时均匀分布（每个渠道有效权重 100），避免返回 nil 造成 503
	//   - 平均 weight < 10 时放大 smoothingFactor，保证加权随机的分辨率
	sumWeight := 0
	targetIds := make([]int, 0, len(filteredIds))
	for _, id := range filteredIds {
		ch, ok := channelsById[id]
		if !ok {
			continue
		}
		if computeEffectivePriority(ch, estTokens) == targetPriority {
			sumWeight += ch.GetWeight()
			targetIds = append(targetIds, id)
		}
	}
	if len(targetIds) == 0 {
		return nil, fallback, nil
	}

	smoothingFactor := 1
	smoothingAdjustment := 0
	if sumWeight == 0 {
		sumWeight = len(targetIds) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetIds) < 10 {
		smoothingFactor = 100
	}

	totalWeight := sumWeight * smoothingFactor
	weight := common.GetRandomInt(totalWeight)
	var selectedId int
	for _, id := range targetIds {
		ch, ok := channelsById[id]
		if !ok {
			continue
		}
		weight -= ch.GetWeight()*smoothingFactor + smoothingAdjustment
		if weight < 0 {
			selectedId = id
			break
		}
	}
	if selectedId == 0 && len(targetIds) > 0 {
		selectedId = targetIds[0]
	}

	ch, ok := channelsById[selectedId]
	if !ok {
		return nil, fallback, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", selectedId)
	}
	return ch, fallback, nil
}

// loadChannelsByIds 批量加载候选 Channel 行（含 MaxTokens / TokenTiers），返回 id -> *Channel map。
// 找不到的 id 不放入 map，由调用方当作一致性错误处理。
func loadChannelsByIds(ids []int) (map[int]*Channel, error) {
	if len(ids) == 0 {
		return make(map[int]*Channel), nil
	}
	var channels []*Channel
	err := DB.Where("id IN ?", ids).Find(&channels).Error
	if err != nil {
		return nil, err
	}
	m := make(map[int]*Channel, len(channels))
	for _, ch := range channels {
		m[ch.Id] = ch
	}
	return m, nil
}

// filterChannelsByMaxTokensFromMap 是 filterChannelsByMaxTokens 的 DB 路径版本，
// 直接基于已加载的 channelsById map 进行软过滤，避免依赖全局 channelsIDM（仅内存缓存路径可用）。
func filterChannelsByMaxTokensFromMap(ids []int, channelsById map[int]*Channel, estTokens int) (filtered []int, fallback bool) {
	if estTokens <= 0 || len(ids) == 0 {
		return ids, false
	}
	filtered = make([]int, 0, len(ids))
	for _, id := range ids {
		ch, ok := channelsById[id]
		if !ok {
			// 保留未知 id，让下游发出一致性错误，行为与内存缓存路径一致
			filtered = append(filtered, id)
			continue
		}
		if ch.MaxTokens > 0 && estTokens > ch.MaxTokens {
			continue
		}
		filtered = append(filtered, id)
	}
	if len(filtered) == 0 {
		return ids, true
	}
	return filtered, false
}

// filterAbilitiesByRequestPath restricts candidates by request path for the DB
// (non-memory-cache) selection path. Only Advanced Custom (type 58) channels are
// path-checked: kept only when one of their routes matches requestPath; all other
// channel types always pass. When requestPath is empty, filtering is skipped.
func filterAbilitiesByRequestPath(abilities []Ability, requestPath string) []Ability {
	if requestPath == "" || len(abilities) == 0 {
		return abilities
	}

	channelIds := make([]int, 0, len(abilities))
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, ok := seen[ability.ChannelId]; ok {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIds = append(channelIds, ability.ChannelId)
	}

	var channels []*Channel
	if err := DB.Where("id IN ?", channelIds).Find(&channels).Error; err != nil {
		// On error, fall back to unfiltered candidates to avoid blocking selection
		return abilities
	}

	advancedConfigs := make(map[int]*dto.AdvancedCustomConfig)
	for _, channel := range channels {
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			advancedConfigs[channel.Id] = channel.GetOtherSettings().AdvancedCustom
		}
	}

	filtered := make([]Ability, 0, len(abilities))
	for _, ability := range abilities {
		config, isAdvancedCustom := advancedConfigs[ability.ChannelId]
		if !isAdvancedCustom {
			filtered = append(filtered, ability)
			continue
		}
		if config != nil && config.SupportsPath(requestPath) {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
