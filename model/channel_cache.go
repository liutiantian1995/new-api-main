package model

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var group2model2channels map[string]map[string][]int // enabled channel
var channelsIDM map[int]*Channel                     // all channels include disabled
// channel2advancedCustomConfig caches parsed Advanced Custom (type 58) configs so
// path-aware selection avoids re-parsing JSON per request. Refreshed on full sync.
var channel2advancedCustomConfig map[int]*dto.AdvancedCustomConfig
var channelSyncLock sync.RWMutex

// filterChannelsByMaxTokens applies the soft max_tokens filter.
// Returns the filtered slice and a fallback flag. When estTokens > 0 and the
// filter removes every candidate, fallback is true and the original slice is
// returned so the caller can fall back to the full set. Channels with
// MaxTokens == 0 (unconfigured) always pass.
func filterChannelsByMaxTokens(channelIds []int, estTokens int) (filtered []int, fallback bool) {
	if estTokens <= 0 || len(channelIds) == 0 {
		return channelIds, false
	}
	filtered = make([]int, 0, len(channelIds))
	for _, id := range channelIds {
		ch, ok := channelsIDM[id]
		if !ok {
			// keep unknown ids so downstream emits the same consistency error as before
			filtered = append(filtered, id)
			continue
		}
		if ch.MaxTokens > 0 && estTokens > ch.MaxTokens {
			continue
		}
		filtered = append(filtered, id)
	}
	if len(filtered) == 0 {
		return channelIds, true
	}
	return filtered, false
}

// computeEffectivePriority returns base priority plus the sum of priority boosts
// from token tiers whose max_tokens >= estTokens. estTokens <= 0 means no boost.
func computeEffectivePriority(ch *Channel, estTokens int) int64 {
	p, _ := computeEffectivePriorityWithTier(ch, estTokens)
	return p
}

// computeEffectivePriorityWithTier 返回 effective priority 和命中的最大 tier
// （max_tokens 最大的那档）。未命中任何 tier 时返回 (base, nil)。
func computeEffectivePriorityWithTier(ch *Channel, estTokens int) (int64, *TokenTier) {
	base := ch.GetPriority()
	if estTokens <= 0 || len(ch.TokenTiers) == 0 {
		return base, nil
	}
	var boost int64
	var topTier *TokenTier
	for i := range ch.TokenTiers {
		tier := &ch.TokenTiers[i]
		if tier.MaxTokens > 0 && estTokens <= tier.MaxTokens {
			boost += tier.PriorityBoost
			if topTier == nil || tier.MaxTokens > topTier.MaxTokens {
				topTier = tier
			}
		}
	}
	return base + boost, topTier
}

func InitChannelCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newChannelId2channel := make(map[int]*Channel)
	newChannel2advancedCustomConfig := make(map[int]*dto.AdvancedCustomConfig)
	var channels []*Channel
	DB.Find(&channels)
	for _, channel := range channels {
		newChannelId2channel[channel.Id] = channel
		if channel.Type == constant.ChannelTypeAdvancedCustom {
			if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
				newChannel2advancedCustomConfig[channel.Id] = config
			}
		}
	}
	var abilities []*Ability
	DB.Find(&abilities)
	groups := make(map[string]bool)
	for _, ability := range abilities {
		groups[ability.Group] = true
	}
	newGroup2model2channels := make(map[string]map[string][]int)
	for group := range groups {
		newGroup2model2channels[group] = make(map[string][]int)
	}
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue // skip disabled channels
		}
		groups := strings.Split(channel.Group, ",")
		for _, group := range groups {
			models := strings.Split(channel.Models, ",")
			for _, model := range models {
				if _, ok := newGroup2model2channels[group][model]; !ok {
					newGroup2model2channels[group][model] = make([]int, 0)
				}
				newGroup2model2channels[group][model] = append(newGroup2model2channels[group][model], channel.Id)
			}
		}
	}

	// sort by priority
	for group, model2channels := range newGroup2model2channels {
		for model, channels := range model2channels {
			sort.Slice(channels, func(i, j int) bool {
				return newChannelId2channel[channels[i]].GetPriority() > newChannelId2channel[channels[j]].GetPriority()
			})
			newGroup2model2channels[group][model] = channels
		}
	}

	channelSyncLock.Lock()
	group2model2channels = newGroup2model2channels
	//channelsIDM = newChannelId2channel
	for i, channel := range newChannelId2channel {
		if channel.ChannelInfo.IsMultiKey {
			channel.Keys = channel.GetKeys()
			if channel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
				if oldChannel, ok := channelsIDM[i]; ok {
					// 存在旧的渠道，如果是多key且轮询，保留轮询索引信息
					if oldChannel.ChannelInfo.IsMultiKey && oldChannel.ChannelInfo.MultiKeyMode == constant.MultiKeyModePolling {
						channel.ChannelInfo.MultiKeyPollingIndex = oldChannel.ChannelInfo.MultiKeyPollingIndex
					}
				}
			}
		}
	}
	channelsIDM = newChannelId2channel
	channel2advancedCustomConfig = newChannel2advancedCustomConfig
	channelSyncLock.Unlock()
	common.SysLog("channels synced from database")
}

func SyncChannelCache(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing channels from database")
		InitChannelCache()
	}
}

// GetRandomSatisfiedChannel selects a channel for the given group/model/retry.
// The returned bool is true when max_tokens soft-filtering removed every
// candidate and the selection fell back to the full set; callers (distributor)
// use it to set the X-Token-Routing-Fallback response header.
func GetRandomSatisfiedChannel(group string, model string, retry int, requestPath string, estTokens int) (channel *Channel, fallback bool, err error) {
	// if memory cache is disabled, get channel directly from database
	if !common.MemoryCacheEnabled {
		ch, fb, dbErr := GetChannel(group, model, retry, requestPath, estTokens)
		return ch, fb, dbErr
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	// First, try to find channels with the exact model name.
	channels := filterChannelsByRequestPath(group2model2channels[group][model], requestPath)

	// If no channels found, try to find channels with the normalized model name.
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(model)
		channels = filterChannelsByRequestPath(group2model2channels[group][normalizedModel], requestPath)
	}

	if len(channels) == 0 {
		return nil, false, nil
	}

	if len(channels) == 1 {
		if ch, ok := channelsIDM[channels[0]]; ok {
			return ch, false, nil
		}
		return nil, false, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
	}

	// token-aware routing：先按 max_tokens 软过滤，再按 effective_priority 分 tier。
	// 当 estTokens > 0 且过滤掉全部候选时，回退到原始全集合（让上游自身容量策略处理）。
	filtered, fallback := filterChannelsByMaxTokens(channels, estTokens)
	if fallback {
		// 全部被 max_tokens 软过滤掉 → 回退到原始全集合，并把 fallback 信号返回给调用方。
	} else {
		channels = filtered
	}

	// effective_priority = base_priority + Σ(token_tier.priority_boost where estTokens ≤ tier.max_tokens)
	// 同 base 的渠道可能因 boost 不同而落到不同 effective tier。
	uniqueEffectivePriorities := make(map[int64]bool)
	for _, channelId := range channels {
		ch, ok := channelsIDM[channelId]
		if !ok {
			return nil, false, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
		uniqueEffectivePriorities[computeEffectivePriority(ch, estTokens)] = true
	}
	var sortedUniquePriorities []int64
	for p := range uniqueEffectivePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, p)
	}
	sort.Slice(sortedUniquePriorities, func(i, j int) bool { return sortedUniquePriorities[i] > sortedUniquePriorities[j] })

	if retry >= len(sortedUniquePriorities) {
		retry = len(sortedUniquePriorities) - 1
	}
	targetPriority := sortedUniquePriorities[retry]

	// get the priority for the given retry number
	var sumWeight = 0
	var targetChannels []*Channel
	for _, channelId := range channels {
		if ch, ok := channelsIDM[channelId]; ok {
			if computeEffectivePriority(ch, estTokens) == targetPriority {
				sumWeight += ch.GetWeight()
				targetChannels = append(targetChannels, ch)
			}
		} else {
			return nil, false, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelId)
		}
	}

	if len(targetChannels) == 0 {
		return nil, false, errors.New(fmt.Sprintf("no channel found, group: %s, model: %s, priority: %d", group, model, targetPriority))
	}

	// smoothing factor and adjustment
	smoothingFactor := 1
	smoothingAdjustment := 0

	if sumWeight == 0 {
		// when all channels have weight 0, set sumWeight to the number of channels and set smoothing adjustment to 100
		// each channel's effective weight = 100
		sumWeight = len(targetChannels) * 100
		smoothingAdjustment = 100
	} else if sumWeight/len(targetChannels) < 10 {
		// when the average weight is less than 10, set smoothing factor to 100
		smoothingFactor = 100
	}

	// Calculate the total weight of all channels up to endIdx
	totalWeight := sumWeight * smoothingFactor

	// Generate a random value in the range [0, totalWeight)
	randomWeight := rand.Intn(totalWeight)

	// Find a channel based on its weight
	// 注意：fallback 变量由 filterChannelsByMaxTokens 设定后在此循环内不可变，
	// 下面的 return 把该信号透传给调用方（distributor 据此设置响应头）。
	// 任何后续重构都不得在此循环内重赋 fallback，否则会破坏 fallback 语义。
	for _, ch := range targetChannels {
		randomWeight -= ch.GetWeight()*smoothingFactor + smoothingAdjustment
		if randomWeight < 0 {
			return ch, fallback, nil
		}
	}
	// return null if no channel is not found
	return nil, fallback, errors.New("channel not found")
}

// filterChannelsByRequestPath restricts candidates by request path. Only Advanced
// Custom (type 58) channels are path-checked: they are kept only when one of their
// configured routes matches requestPath. All other channel types always pass.
// When requestPath is empty (non-relay callers) filtering is skipped.
// Caller must hold channelSyncLock (read lock). The cached slice is never mutated.
func filterChannelsByRequestPath(channels []int, requestPath string) []int {
	if requestPath == "" || len(channels) == 0 {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelId := range channels {
		channel, ok := channelsIDM[channelId]
		if !ok {
			// keep it so the downstream consistency error is raised as before
			filtered = append(filtered, channelId)
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			filtered = append(filtered, channelId)
			continue
		}
		if config := channel2advancedCustomConfig[channelId]; config != nil && config.SupportsPath(requestPath) {
			filtered = append(filtered, channelId)
		}
	}
	return filtered
}

func CacheGetChannel(id int) (*Channel, error) {
	if !common.MemoryCacheEnabled {
		return GetChannelById(id, true)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return c, nil
}

func CacheGetChannelInfo(id int) (*ChannelInfo, error) {
	if !common.MemoryCacheEnabled {
		channel, err := GetChannelById(id, true)
		if err != nil {
			return nil, err
		}
		return &channel.ChannelInfo, nil
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	c, ok := channelsIDM[id]
	if !ok {
		return nil, fmt.Errorf("渠道# %d，已不存在", id)
	}
	return &c.ChannelInfo, nil
}

func CacheUpdateChannelStatus(id int, status int) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel, ok := channelsIDM[id]; ok {
		channel.Status = status
	}
	if status != common.ChannelStatusEnabled {
		// delete the channel from group2model2channels
		for group, model2channels := range group2model2channels {
			for model, channels := range model2channels {
				for i, channelId := range channels {
					if channelId == id {
						// remove the channel from the slice
						group2model2channels[group][model] = append(channels[:i], channels[i+1:]...)
						break
					}
				}
			}
		}
	}
}

func CacheUpdateChannel(channel *Channel) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	if channel == nil {
		return
	}

	if channelsIDM == nil {
		channelsIDM = make(map[int]*Channel)
	}
	if oldChannel, ok := channelsIDM[channel.Id]; ok {
		logger.LogDebug(nil, "CacheUpdateChannel before: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, oldChannel.ChannelInfo.MultiKeyPollingIndex)
	}
	channelsIDM[channel.Id] = channel
	logger.LogDebug(nil, "CacheUpdateChannel after: id=%d, name=%s, status=%d, polling_index=%d", channel.Id, channel.Name, channel.Status, channel.ChannelInfo.MultiKeyPollingIndex)
}
