package knowledge

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// BuildKnowledgeRetrieveChain 编译「查询字符串 → 文档列表」的 Eino Chain（MultiQuery → 向量 → 重排 → 后处理）。
func BuildKnowledgeRetrieveChain(ctx context.Context, r *Retriever) (compose.Runnable[string, []*schema.Document], error) {
	if r == nil {
		return nil, fmt.Errorf("retriever is nil")
	}
	ch := compose.NewChain[string, []*schema.Document]()
	ch.AppendRetriever(r.AsEinoRetriever())
	return ch.Compile(ctx)
}

// CompileRetrieveChain 等价于 [BuildKnowledgeRetrieveChain](ctx, r)。
func (r *Retriever) CompileRetrieveChain(ctx context.Context) (compose.Runnable[string, []*schema.Document], error) {
	return BuildKnowledgeRetrieveChain(ctx, r)
}
