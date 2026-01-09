package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"one-api/common/config"
	"one-api/common/logger"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

type OAuth2Config struct {
	OAuth2Config *oauth2.Config
	UserInfoURL  string
	LoginURL     func(state string) string
}

type OAuth2UserInfo struct {
	Id          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

var oauth2ConfigInstance *OAuth2Config

// 初始化OAuth2配置
func InitOAuth2Config() error {
	if !config.OAuth2AuthEnabled {
		return nil
	}

	logger.SysLog("OAuth2功能启用")

	if config.OAuth2ClientId == "" || config.OAuth2ClientSecret == "" {
		return errors.New("OAuth2 ClientId 或 ClientSecret 未配置")
	}

	if config.OAuth2AuthorizeUrl == "" || config.OAuth2TokenUrl == "" {
		return errors.New("OAuth2 AuthorizeUrl 或 TokenUrl 未配置")
	}

	if config.OAuth2UserInfoUrl == "" {
		return errors.New("OAuth2 UserInfoUrl 未配置")
	}

	if config.ServerAddress == "" {
		return errors.New("ServerAddress 未配置")
	}

	scopes := []string{}
	if config.OAuth2Scopes != "" {
		scopes = strings.Split(config.OAuth2Scopes, ",")
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.OAuth2ClientId,
		ClientSecret: config.OAuth2ClientSecret,
		RedirectURL:  config.ServerAddress + "/oauth/oauth2",
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.OAuth2AuthorizeUrl,
			TokenURL: config.OAuth2TokenUrl,
		},
		Scopes: scopes,
	}

	oauth2ConfigInstance = &OAuth2Config{
		OAuth2Config: oauth2Config,
		UserInfoURL:  config.OAuth2UserInfoUrl,
		LoginURL: func(state string) string {
			return oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
		},
	}

	return nil
}

// 确保线程安全
var mu sync.Mutex

// 获取 OAuth2Config 实例，如果未初始化则进行初始化
func GetOAuth2ConfigInstance() (*OAuth2Config, error) {
	mu.Lock()
	defer mu.Unlock()
	if oauth2ConfigInstance == nil {
		err := InitOAuth2Config()
		if err != nil {
			return nil, err
		}
	}
	return oauth2ConfigInstance, nil
}

// 重置 OAuth2Config 实例，用于配置更新后重新初始化
func ResetOAuth2Config() {
	mu.Lock()
	defer mu.Unlock()
	oauth2ConfigInstance = nil
}

// GetUserInfo 通过access token获取用户信息
func (c *OAuth2Config) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuth2UserInfo, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	// 设置Authorization头
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.SysError("OAuth2获取用户信息失败: " + err.Error())
		return nil, errors.New("无法连接至OAuth2服务器，请稍后重试")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.SysError("OAuth2获取用户信息失败，状态码: " + resp.Status + ", 响应: " + string(body))
		return nil, errors.New("获取用户信息失败，状态码: " + resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析为通用map
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	userInfo := &OAuth2UserInfo{}

	// 根据配置的字段名获取用户信息
	userInfo.Id = getStringField(rawData, config.OAuth2UserIdField)
	userInfo.Username = getStringField(rawData, config.OAuth2UsernameField)
	userInfo.Email = getStringField(rawData, config.OAuth2EmailField)
	userInfo.DisplayName = getStringField(rawData, config.OAuth2DisplayNameField)

	if userInfo.Id == "" {
		return nil, errors.New("OAuth2用户信息中缺少ID字段")
	}

	return userInfo, nil
}

// 从map中获取字符串字段值，支持嵌套字段（如 "data.user.id"）
func getStringField(data map[string]interface{}, fieldPath string) string {
	if fieldPath == "" {
		return ""
	}

	fields := strings.Split(fieldPath, ".")
	current := data

	for i, field := range fields {
		if val, ok := current[field]; ok {
			if i == len(fields)-1 {
				// 最后一个字段，返回字符串值
				return toString(val)
			}
			// 不是最后一个字段，继续向下遍历
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return ""
			}
		} else {
			return ""
		}
	}
	return ""
}

// 将interface{}转换为字符串
func toString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		// JSON中的数字默认解析为float64
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
