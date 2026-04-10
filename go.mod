module cyberstrike-ai

// 若 go mod download 超时，可执行: go env -w GOPROXY=https://goproxy.cn,direct
// 或使用 scripts/bootstrap-go.sh

go 1.24.0

toolchain go1.24.4

require (
	github.com/bytedance/sonic v1.15.0
	github.com/cloudwego/eino v0.8.4
	github.com/cloudwego/eino-ext/components/model/openai v0.1.10
	github.com/creack/pty v1.1.24
	github.com/eino-contrib/jsonschema v1.0.3
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.0
	github.com/larksuite/oapi-sdk-go/v3 v3.4.22
	github.com/mattn/go-sqlite3 v1.14.18
	github.com/modelcontextprotocol/go-sdk v1.2.0
	github.com/open-dingtalk/dingtalk-stream-sdk-go v0.9.1
	github.com/pkoukk/tiktoken-go v0.1.8
	go.uber.org/zap v1.26.0
	golang.org/x/time v0.14.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/cloudwego/eino-ext/libs/acl/openai v0.1.14 // indirect
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/evanphx/json-patch v0.5.2 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/jsonschema-go v0.3.0 // indirect
	github.com/goph/emperror v0.17.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/meguminnnnnnnnn/go-openai v0.1.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nikolalohinski/gonja v1.5.3 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/slongfield/pyfmt v0.0.0-20220222012616-ea85ff4c361f // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/yargevad/filepathx v1.0.0 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/arch v0.11.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/exp v0.0.0-20230713183714-613f0c0eb8a1 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)

// 修复钉钉 Stream SDK 在长连接断开（熄屏/网络中断）后 "panic: send on closed channel" 问题
// 详见: https://github.com/open-dingtalk/dingtalk-stream-sdk-go/issues/28
replace github.com/open-dingtalk/dingtalk-stream-sdk-go => github.com/uouuou/dingtalk-stream-sdk-go v0.0.0-20250626025113-079132acc406
