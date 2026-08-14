package handler

import (
	"github.com/mojocn/base64Captcha"
)

// CaptchaService 验证码生成与校验服务（基于 base64Captcha）。
type CaptchaService struct {
	store  base64Captcha.Store
	driver base64Captcha.Driver
}

// NewCaptchaService 创建默认数字验证码服务（内存存储，自动过期）。
func NewCaptchaService() *CaptchaService {
	// 高度 80px，宽度 240px，5 位数字，干扰线密度 0.7，最大倾斜角 80
	driver := base64Captcha.NewDriverDigit(80, 240, 5, 0.7, 80)
	return &CaptchaService{
		store:  base64Captcha.DefaultMemStore,
		driver: driver,
	}
}

// Generate 生成一个新验证码，返回 id 和 base64 编码的图片字符串。
func (s *CaptchaService) Generate() (id, b64s string, err error) {
	c := base64Captcha.NewCaptcha(s.driver, s.store)
	id, b64s, _, err = c.Generate()
	return
}

// Verify 校验验证码，校验后该 id 立即失效（clear=true）。
func (s *CaptchaService) Verify(id, answer string) bool {
	return s.store.Verify(id, answer, true)
}
