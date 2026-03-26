package main

import "time"

type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	TS   int64       `json:"ts"`
	Data interface{} `json:"data,omitempty"`
}

type User struct {
	ID               int64  `json:"id"`
	Name             string `json:"name,omitempty"`
	User             string `json:"user"`
	Pwd              string `json:"-"`
	RoleID           int    `json:"role_id,omitempty"`
	ExpTime          int64  `json:"expTime"`
	Flow             int64  `json:"flow"`
	InFlow           int64  `json:"inFlow"`
	OutFlow          int64  `json:"outFlow"`
	FlowResetTime    int64  `json:"flowResetTime"`
	Num              int    `json:"num"`
	CreatedTime      int64  `json:"createdTime"`
	UpdatedTime      int64  `json:"updatedTime"`
	Status           int    `json:"status"`
	SubscriptionToken string `json:"subscriptionToken,omitempty"`
}

type UserTunnel struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"userId"`
	TunnelID      int64  `json:"tunnelId"`
	Flow          int64  `json:"flow"`
	InFlow        int64  `json:"inFlow"`
	OutFlow       int64  `json:"outFlow"`
	FlowResetTime int64  `json:"flowResetTime"`
	ExpTime       int64  `json:"expTime"`
	SpeedID       *int64 `json:"speedId"`
	Num           int    `json:"num"`
	Status        int    `json:"status"`
	TunnelName    string `json:"tunnelName,omitempty"`
	TunnelFlow    int    `json:"tunnelFlow,omitempty"`
	SpeedLimitName string `json:"speedLimitName,omitempty"`
	Speed         *int   `json:"speed,omitempty"`
}

type Node struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Secret      string `json:"secret,omitempty"`
	IP          string `json:"ip"`
	ServerIP    string `json:"serverIp"`
	Version     string `json:"version,omitempty"`
	PortSta     int    `json:"portSta"`
	PortEnd     int    `json:"portEnd"`
	HTTP        int    `json:"http"`
	TLS         int    `json:"tls"`
	Socks       int    `json:"socks"`
	CreatedTime int64  `json:"createdTime"`
	UpdatedTime int64  `json:"updatedTime"`
	Status      int    `json:"status"`
}

type Tunnel struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	InNodeID      int64   `json:"inNodeId"`
	InIP          string  `json:"inIp"`
	OutNodeID     int64   `json:"outNodeId"`
	OutIP         string  `json:"outIp"`
	Type          int     `json:"type"`
	Flow          int     `json:"flow"`
	Protocol      string  `json:"protocol"`
	TrafficRatio  float64 `json:"trafficRatio"`
	TCPListenAddr string  `json:"tcpListenAddr"`
	UDPListenAddr string  `json:"udpListenAddr"`
	InterfaceName string  `json:"interfaceName,omitempty"`
	CreatedTime   int64   `json:"createdTime"`
	UpdatedTime   int64   `json:"updatedTime"`
	Status        int     `json:"status"`
	InNodePortSta int     `json:"inNodePortSta,omitempty"`
	InNodePortEnd int     `json:"inNodePortEnd,omitempty"`
}

type Forward struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"userId"`
	UserName      string `json:"userName,omitempty"`
	Name          string `json:"name"`
	TunnelID      int64  `json:"tunnelId"`
	InPort        int    `json:"inPort"`
	OutPort       int    `json:"outPort"`
	RemoteAddr    string `json:"remoteAddr"`
	InterfaceName string `json:"interfaceName,omitempty"`
	Strategy      string `json:"strategy"`
	InFlow        int64  `json:"inFlow"`
	OutFlow       int64  `json:"outFlow"`
	Inx           int    `json:"inx"`
	CreatedTime   int64  `json:"createdTime"`
	UpdatedTime   int64  `json:"updatedTime"`
	Status        int    `json:"status"`
	TunnelName    string `json:"tunnelName,omitempty"`
	InIP          string `json:"inIp,omitempty"`
	OutIP         string `json:"outIp,omitempty"`
	Type          int    `json:"type,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

type SpeedLimit struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Speed      int    `json:"speed"`
	TunnelID   int64  `json:"tunnelId"`
	TunnelName string `json:"tunnelName"`
	CreatedTime int64 `json:"createdTime"`
	UpdatedTime int64 `json:"updatedTime"`
	Status      int   `json:"status"`
}

type StatisticsFlow struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	Flow        int64  `json:"flow"`
	TotalFlow   int64  `json:"totalFlow"`
	Time        string `json:"time"`
	CreatedTime int64  `json:"createdTime"`
}

type ViteConfig struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Time  int64  `json:"time"`
}

type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	CaptchaID string `json:"captchaId"`
}

type LoginPayload struct {
	Token                 string `json:"token"`
	RoleID                int    `json:"role_id"`
	Name                  string `json:"name"`
	RequirePasswordChange bool   `json:"requirePasswordChange"`
}

type UserDTO struct {
	User          string `json:"user"`
	Pwd           string `json:"pwd"`
	Flow          int64  `json:"flow"`
	Num           int    `json:"num"`
	ExpTime       int64  `json:"expTime"`
	FlowResetTime int64  `json:"flowResetTime"`
	Status        *int   `json:"status"`
}

type UserUpdateDTO struct {
	ID            int64   `json:"id"`
	User          string  `json:"user"`
	Pwd           *string `json:"pwd"`
	Flow          int64   `json:"flow"`
	Num           int     `json:"num"`
	ExpTime       int64   `json:"expTime"`
	FlowResetTime int64   `json:"flowResetTime"`
	Status        *int    `json:"status"`
}

type ChangePasswordDTO struct {
	NewUsername     string `json:"newUsername"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type ResetFlowDTO struct {
	ID   int64 `json:"id"`
	Type int   `json:"type"`
}

type NodeDTO struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	ServerIP string `json:"serverIp"`
	PortSta  int    `json:"portSta"`
	PortEnd  int    `json:"portEnd"`
}

type NodeUpdateDTO struct {
	ID       int64 `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	ServerIP string `json:"serverIp"`
	PortSta  int    `json:"portSta"`
	PortEnd  int    `json:"portEnd"`
	HTTP     *int   `json:"http"`
	TLS      *int   `json:"tls"`
	Socks    *int   `json:"socks"`
}

type TunnelDTO struct {
	Name          string   `json:"name"`
	InNodeID      int64    `json:"inNodeId"`
	OutNodeID     *int64   `json:"outNodeId"`
	Type          int      `json:"type"`
	Flow          int      `json:"flow"`
	TrafficRatio  *float64 `json:"trafficRatio"`
	InterfaceName string   `json:"interfaceName"`
	Protocol      string   `json:"protocol"`
	TCPListenAddr string   `json:"tcpListenAddr"`
	UDPListenAddr string   `json:"udpListenAddr"`
}

type TunnelUpdateDTO struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Flow          int      `json:"flow"`
	TrafficRatio  *float64 `json:"trafficRatio"`
	Protocol      string   `json:"protocol"`
	TCPListenAddr string   `json:"tcpListenAddr"`
	UDPListenAddr string   `json:"udpListenAddr"`
	InterfaceName *string  `json:"interfaceName"`
}

type UserTunnelDTO struct {
	UserID        int64  `json:"userId"`
	TunnelID      int64  `json:"tunnelId"`
	Flow          int64  `json:"flow"`
	Num           int    `json:"num"`
	FlowResetTime int64  `json:"flowResetTime"`
	ExpTime       int64  `json:"expTime"`
	SpeedID       *int64 `json:"speedId"`
}

type UserTunnelQueryDTO struct {
	UserID int64 `json:"userId"`
}

type UserTunnelUpdateDTO struct {
	ID            int64  `json:"id"`
	Flow          int64  `json:"flow"`
	Num           int    `json:"num"`
	FlowResetTime int64  `json:"flowResetTime"`
	ExpTime       int64  `json:"expTime"`
	Status        int    `json:"status"`
	SpeedID       *int64 `json:"speedId"`
}

type ForwardDTO struct {
	Name          string `json:"name"`
	TunnelID      int64  `json:"tunnelId"`
	RemoteAddr    string `json:"remoteAddr"`
	Strategy      string `json:"strategy"`
	InPort        *int   `json:"inPort"`
	InterfaceName string `json:"interfaceName"`
}

type ForwardUpdateDTO struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"userId"`
	Name          string `json:"name"`
	TunnelID      int64  `json:"tunnelId"`
	RemoteAddr    string `json:"remoteAddr"`
	Strategy      string `json:"strategy"`
	InPort        *int   `json:"inPort"`
	InterfaceName *string `json:"interfaceName"`
}

type SpeedLimitDTO struct {
	Name       string `json:"name"`
	Speed      int    `json:"speed"`
	TunnelID   int64  `json:"tunnelId"`
	TunnelName string `json:"tunnelName"`
}

type SpeedLimitUpdateDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Speed      int    `json:"speed"`
	TunnelID   int64  `json:"tunnelId"`
	TunnelName string `json:"tunnelName"`
}

type FlowDTO struct {
	N string `json:"n"`
	U int64  `json:"u"`
	D int64  `json:"d"`
}

type GostResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

type UserPackageResponse struct {
	UserInfo          UserPackageUserInfo    `json:"userInfo"`
	TunnelPermissions []UserTunnel           `json:"tunnelPermissions"`
	Forwards          []Forward              `json:"forwards"`
	StatisticsFlows   []StatisticsFlow       `json:"statisticsFlows"`
}

type UserPackageUserInfo struct {
	ID                int64  `json:"id"`
	Name              string `json:"name,omitempty"`
	User              string `json:"user"`
	Status            int    `json:"status"`
	Flow              int64  `json:"flow"`
	InFlow            int64  `json:"inFlow"`
	OutFlow           int64  `json:"outFlow"`
	Num               int    `json:"num"`
	ExpTime           int64  `json:"expTime"`
	FlowResetTime     int64  `json:"flowResetTime"`
	CreatedTime       int64  `json:"createdTime"`
	UpdatedTime       int64  `json:"updatedTime"`
	SubscriptionToken string `json:"subscriptionToken,omitempty"`
}

type CaptchaVerifyRequest struct {
	ID   string      `json:"id"`
	Data interface{} `json:"data"`
}

type CaptchaGenerateResponse struct {
	ID      string                 `json:"id"`
	Captcha map[string]interface{} `json:"captcha"`
}

type EncryptedEnvelope struct {
	Encrypted bool   `json:"encrypted"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

type NodeCommand struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"requestId,omitempty"`
}

type NodeCommandResponse struct {
	Type      string      `json:"type"`
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"requestId,omitempty"`
}

type SystemInfo struct {
	Uptime           uint64  `json:"uptime"`
	BytesReceived    uint64  `json:"bytes_received"`
	BytesTransmitted uint64  `json:"bytes_transmitted"`
	CPUUsage         float64 `json:"cpu_usage"`
	MemoryUsage      float64 `json:"memory_usage"`
}

type Ticket struct {
	Value     string
	UserID    int64
	RoleID    int
	ExpiresAt time.Time
}
