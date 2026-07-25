package controller

import (
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}

// GroupDetail is a single group entry returned by GetGroupDetails.
type GroupDetail struct {
	Name  string  `json:"name"`
	Ratio float64 `json:"ratio"`
	Desc  string  `json:"desc"`
}

// GetGroupDetails returns all configured groups with ratio and description,
// sorted by name. Intended for admin surfaces (e.g. batch user creation)
// where the full system group list is required regardless of any single
// user's usable-group set.
func GetGroupDetails(c *gin.Context) {
	groupRatio := ratio_setting.GetGroupRatioCopy()
	details := make([]GroupDetail, 0, len(groupRatio))
	for name, ratio := range groupRatio {
		details = append(details, GroupDetail{
			Name:  name,
			Ratio: ratio,
			Desc:  setting.GetUsableGroupDescription(name),
		})
	}
	sort.Slice(details, func(i, j int) bool {
		return details[i].Name < details[j].Name
	})
	common.ApiSuccess(c, details)
}
