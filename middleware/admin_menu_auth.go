package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func AdminMenuAuth(menuKey string) func(c *gin.Context) {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		role := c.GetInt("role")
		if !c.GetBool("use_access_token") && userId > 0 {
			user, err := model.GetUserById(userId, false)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			role = user.Role
			if user.Status == common.UserStatusDisabled || role < common.RoleAdminUser {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "无权进行此操作，权限不足",
				})
				c.Abort()
				return
			}
			c.Set("role", role)
		}
		ok, err := model.UserHasAdminMenuPermission(userId, role, menuKey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
		if !ok {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无权进行此操作，菜单权限不足",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
