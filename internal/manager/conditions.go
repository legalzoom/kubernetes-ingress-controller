package manager

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	ctrlutils "github.com/kong/kubernetes-ingress-controller/v2/internal/controllers/utils"
)

func ShouldEnableCRDController(gvr schema.GroupVersionResource, restMapper meta.RESTMapper) bool {
	if !ctrlutils.CRDExists(restMapper, gvr) {
		ctrl.Log.WithName("controllers").WithName("crdCondition").
			Info(fmt.Sprintf("Disabling controller for Group=%s, Resource=%s due to missing CRD", gvr.GroupVersion(), gvr.Resource))
		return false
	}
	return true
}
