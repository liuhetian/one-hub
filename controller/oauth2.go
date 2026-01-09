package controller

import (
	"context"
	"errors"
	"net/http"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/oauth2"
	"one-api/common/utils"
	"one-api/model"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OAuth2Endpoint 获取OAuth2登录URL
func OAuth2Endpoint(c *gin.Context) {
	if !config.OAuth2AuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员未开启通过OAuth2登录",
			"success": false,
		})
		return
	}

	oauth2Config, err := oauth2.GetOAuth2ConfigInstance()
	if err != nil {
		logger.SysError("获取 OAuth2 配置失败, err: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": "获取 OAuth2 配置失败: " + err.Error(),
			"success": false,
		})
		return
	}

	session := sessions.Default(c)
	state := utils.GetRandomString(12)
	session.Set("oauth_state", state)
	loginURL := oauth2Config.LoginURL(state)
	err = session.Save()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    loginURL,
	})
}

// OAuth2Auth 通过OAuth2登录
func OAuth2Auth(c *gin.Context) {
	if !config.OAuth2AuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "管理员未开启通过OAuth2登录",
			"success": false,
		})
		return
	}

	// 验证state参数
	session := sessions.Default(c)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "state is empty or not same",
		})
		return
	}

	// 检查是否是绑定操作
	username := session.Get("username")
	if username != nil {
		OAuth2Bind(c)
		return
	}

	// 获取OAuth2配置
	oauth2Config, err := oauth2.GetOAuth2ConfigInstance()
	if err != nil {
		logger.SysError("获取 OAuth2 配置失败, err: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": "获取 OAuth2 配置失败",
			"success": false,
		})
		return
	}

	// 处理授权码并获取token
	code := c.Query("code")
	ctx := context.Background()
	token, err := oauth2Config.OAuth2Config.Exchange(ctx, code)
	if err != nil {
		logger.SysError("OAuth2 token交换失败: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "OAuth2认证失败: " + err.Error(),
		})
		return
	}

	// 获取用户信息
	userInfo, err := oauth2Config.GetUserInfo(ctx, token)
	if err != nil {
		logger.SysError("OAuth2获取用户信息失败: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取用户信息失败: " + err.Error(),
		})
		return
	}

	// 检测OAuth2用户ID
	if userInfo.Id == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "OAuth2用户信息中没有ID",
		})
		return
	}

	// 初始化用户对象
	user := model.User{
		OAuth2Id: userInfo.Id,
	}

	// 尝试通过OAuth2Id查询用户
	if err = user.FillUserByOAuth2Id(); err == nil {
		if user.Status == config.UserStatusEnabled {
			setupLogin(&user, c)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "用户已被封禁或不存在",
			"success": false,
		})
		return
	}

	// OAuth2Id查询失败，则尝试通过username查询（如果有username）
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.SysError("查询用户错误: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	// 如果有用户名，尝试通过用户名匹配
	if userInfo.Username != "" {
		user.Username = userInfo.Username
		if err = user.FillUserByUsername(); err == nil {
			if user.Status == config.UserStatusEnabled {
				// 如果通过用户名查询用户成功、则补全用户OAuth2 ID并且登录
				user.OAuth2Id = userInfo.Id
				ok := user.Update(false)
				if ok != nil {
					c.JSON(http.StatusOK, gin.H{
						"message": ok.Error(),
						"success": false,
					})
					return
				}
				setupLogin(&user, c)
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message": "用户已被封禁或不存在",
				"success": false,
			})
			return
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.SysError("查询用户错误: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"message": err.Error(),
				"success": false,
			})
			return
		}
	}

	// 用户不存在，尝试注册
	if !config.RegisterEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员关闭了新用户注册",
		})
		return
	}

	// 检测邀请码
	var inviterId int
	affCode := c.Query("aff")
	if affCode != "" {
		inviterId, _ = model.GetUserIdByAffCode(affCode)
	}
	if inviterId > 0 {
		user.InviterId = inviterId
	}

	// 填充用户信息并创建账户
	if userInfo.Username != "" {
		user.Username = userInfo.Username
	} else {
		user.Username = "oauth2_" + userInfo.Id
	}

	// 检查用户名是否已被占用
	if model.IsUsernameAlreadyTaken(user.Username) {
		user.Username = "oauth2_" + strconv.Itoa(model.GetMaxUserId()+1)
	}

	user.OAuth2Id = userInfo.Id

	if userInfo.Email != "" {
		user.Email = userInfo.Email
	}

	if userInfo.DisplayName != "" {
		user.DisplayName = userInfo.DisplayName
	} else if userInfo.Username != "" {
		user.DisplayName = userInfo.Username
	}

	user.Role = config.RoleCommonUser
	user.Status = config.UserStatusEnabled

	if err := user.Insert(inviterId); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	setupLogin(&user, c)
}

// OAuth2Bind 绑定OAuth2账号
func OAuth2Bind(c *gin.Context) {
	if !config.OAuth2AuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启通过OAuth2登录",
		})
		return
	}

	// 获取OAuth2配置
	oauth2Config, err := oauth2.GetOAuth2ConfigInstance()
	if err != nil {
		logger.SysError("获取 OAuth2 配置失败, err: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"message": "获取 OAuth2 配置失败",
			"success": false,
		})
		return
	}

	code := c.Query("code")
	ctx := context.Background()
	token, err := oauth2Config.OAuth2Config.Exchange(ctx, code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "OAuth2认证失败: " + err.Error(),
		})
		return
	}

	userInfo, err := oauth2Config.GetUserInfo(ctx, token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 检查OAuth2 ID是否已被绑定
	if model.IsOAuth2IdAlreadyTaken(userInfo.Id) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该OAuth2账户已被绑定",
		})
		return
	}

	session := sessions.Default(c)
	id := session.Get("id")
	user := model.User{Id: id.(int)}
	err = user.FillUserById()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	user.OAuth2Id = userInfo.Id

	// 如果用户的邮箱为空，且OAuth2用户的邮箱不为空，且邮箱未被注册，则更新用户的邮箱
	if user.Email == "" && userInfo.Email != "" && !model.IsEmailAlreadyTaken(userInfo.Email) {
		user.Email = userInfo.Email
	}

	err = user.Update(false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "bind",
	})
}
