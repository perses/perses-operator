package v1alpha1

import ctrl "sigs.k8s.io/controller-runtime"

func (p *PersesDatasource) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		Complete()
}
