package verify

import (
	"regexp"
	"strings"
)

// IsMobileNumber 检查输入字符串是否为中国大陆合法手机号
// 支持 +86 前缀或纯数字，长度11位，以 1 开头，第二位为 3-9
func IsMobileNumber(s string) bool {
	if s == "" {
		return false
	}

	// 移除可能的空格和 +86 国际区号前缀
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "+86") {
		s = s[3:]
	} else if strings.HasPrefix(s, "86") && len(s) > 2 && s[2] != '6' { // 防止误判如 86123... 这种非+86的情况
		s = s[2:]
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")

	// 匹配一个以 1 开头，第二位是 3 到 9 之间的数字，后面紧跟 9 个数字，并且整个字符串只有这 11 个字符的字符串
	mobileRegex := `^1[3-9]\d{9}$`
	re := regexp.MustCompile(mobileRegex)
	return re.MatchString(s)
}

// IsEmail 检查输入字符串是否为合法邮箱格式
// 基本格式：local@domain.tld
// local: 字母、数字、._%+-，长度1-64
// domain: 字母、数字、-，至少一个点，最后一段为2-6字母
func IsEmail(s string) bool {
	if s == "" {
		return false
	}

	s = strings.TrimSpace(s)
	// 简化但实用的邮箱正则（RFC5322 的简化版）
	// 匹配一个符合以下格式的字符串：
	// 开头是字母、数字或 ._%+- 组成的本地部分（如用户名）
	// 接着是一个 @ 符号
	// 然后是字母、数字、. 或 - 组成的域名
	// 接着是一个 . 点
	// 最后是至少两个字母的顶级域名（如 .com, .org）
	// 整个字符串必须完全匹配，不能有前后缀”
	emailRegex := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(emailRegex)
	return re.MatchString(s)
}
