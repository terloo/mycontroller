# Kubebuilder脚手架

## 目录结构
1. /api：存放crd的go结构体映射
2. /api/crd_types：结构体
3. /api/gourpversion_info：crd的GV
4. /api/zz_generated.deepcoy：使crd实现k8s.io/apimachinery/pkg/runtime.Object接口相关深拷贝方法
5. /controllers/cronjob_controllers：controller的核心reconciler