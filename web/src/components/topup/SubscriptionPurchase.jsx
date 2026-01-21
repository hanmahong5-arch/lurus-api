/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useEffect, useContext } from 'react';
import { useTranslation } from 'react-i18next';
import { Spin, Empty, Modal, Toast } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import CurrentSubscriptionCard from './CurrentSubscriptionCard';
import SubscriptionPlanCard from './SubscriptionPlanCard';
import BillingExplanation from './BillingExplanation';

const SubscriptionPurchase = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);

  const [loading, setLoading] = useState(true);
  const [plans, setPlans] = useState([]);
  const [currentSubscription, setCurrentSubscription] = useState(null);
  const [quotaInfo, setQuotaInfo] = useState(null);
  const [daysRemaining, setDaysRemaining] = useState(0);
  const [hasActive, setHasActive] = useState(false);

  const [purchaseLoading, setPurchaseLoading] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState(null);
  const [confirmVisible, setConfirmVisible] = useState(false);

  // Fetch subscription plans
  const fetchPlans = async () => {
    try {
      const res = await API.get('/api/subscription/plans');
      if (res.data.success) {
        const enabledPlans = (res.data.data || []).filter(p => p.enabled);
        setPlans(enabledPlans);
      }
    } catch (e) {
      console.error('Failed to fetch plans:', e);
    }
  };

  // Fetch current subscription
  const fetchCurrentSubscription = async () => {
    try {
      const res = await API.get('/api/subscription/current');
      if (res.data.success && res.data.data) {
        setCurrentSubscription(res.data.data.subscription);
        setQuotaInfo(res.data.data.quota);
        setDaysRemaining(res.data.data.days_remaining || 0);
        setHasActive(res.data.data.has_active);
      }
    } catch (e) {
      console.error('Failed to fetch current subscription:', e);
    }
  };

  // Refresh user data
  const refreshUserData = async () => {
    try {
      const res = await API.get('/api/user/self');
      if (res.data.success) {
        userDispatch({ type: 'login', payload: res.data.data });
      }
    } catch (e) {
      console.error('Failed to refresh user data:', e);
    }
  };

  useEffect(() => {
    const init = async () => {
      setLoading(true);
      await Promise.all([fetchPlans(), fetchCurrentSubscription()]);
      setLoading(false);
    };
    init();
  }, []);

  // Handle plan selection
  const handleSelectPlan = (plan) => {
    setSelectedPlan(plan);
    setConfirmVisible(true);
  };

  // Handle subscription purchase
  const handlePurchase = async (paymentMethod) => {
    if (!selectedPlan) return;

    setPurchaseLoading(true);
    try {
      const res = await API.post('/api/subscription/create', {
        plan_code: selectedPlan.code,
        payment_method: paymentMethod,
        auto_renew: false,
      });

      if (res.data.success) {
        const { subscription, payment } = res.data.data;
        showSuccess(t('订阅订单已创建，正在跳转支付...'));
        setConfirmVisible(false);

        // Redirect to payment based on method
        if (paymentMethod === 'stripe') {
          // For Stripe, call the subscription payment endpoint
          const payRes = await API.post('/api/user/stripe/subscription/pay', {
            subscription_id: subscription.id,
          });
          if (payRes.data.message === 'success' && payRes.data.data?.pay_link) {
            window.open(payRes.data.data.pay_link, '_blank');
          }
        } else if (paymentMethod === 'creem') {
          const payRes = await API.post('/api/user/creem/subscription/pay', {
            subscription_id: subscription.id,
          });
          if (payRes.data.message === 'success' && payRes.data.data?.checkout_url) {
            window.open(payRes.data.data.checkout_url, '_blank');
          }
        }

        // Refresh data
        await fetchCurrentSubscription();
        await refreshUserData();
      } else {
        showError(res.data.message || t('创建订阅失败'));
      }
    } catch (e) {
      showError(e.message || t('创建订阅失败'));
    } finally {
      setPurchaseLoading(false);
    }
  };

  // Handle cancel auto-renewal
  const handleCancelRenewal = async () => {
    Modal.confirm({
      title: t('确认取消自动续费'),
      content: t('取消后，订阅到期将不会自动续费，但当前订阅权益仍可使用至到期。'),
      onOk: async () => {
        try {
          const res = await API.post('/api/subscription/cancel');
          if (res.data.success) {
            showSuccess(t('已取消自动续费'));
            await fetchCurrentSubscription();
          } else {
            showError(res.data.message);
          }
        } catch (e) {
          showError(t('操作失败'));
        }
      },
    });
  };

  // Get available payment methods
  const getPaymentMethods = () => {
    const methods = [];
    if (statusState?.status?.enable_stripe_topup) {
      methods.push({ key: 'stripe', name: 'Stripe', color: '#6772E5' });
    }
    if (statusState?.status?.enable_creem_topup) {
      methods.push({ key: 'creem', name: 'Creem', color: '#00D4AA' });
    }
    return methods;
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center py-20">
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Current Subscription Status */}
      <CurrentSubscriptionCard
        subscription={currentSubscription}
        quotaInfo={quotaInfo}
        daysRemaining={daysRemaining}
        hasActive={hasActive}
        onCancelRenewal={handleCancelRenewal}
        onRefresh={fetchCurrentSubscription}
      />

      {/* Subscription Plans */}
      <div>
        <h3 className="text-lg font-semibold mb-4">{t('选择订阅套餐')}</h3>
        {plans.length === 0 ? (
          <Empty
            title={t('暂无可用套餐')}
            description={t('管理员尚未配置订阅套餐')}
          />
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {plans.map((plan) => (
              <SubscriptionPlanCard
                key={plan.code}
                plan={plan}
                isCurrent={currentSubscription?.plan_code === plan.code}
                onSelect={() => handleSelectPlan(plan)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Billing Explanation */}
      <BillingExplanation />

      {/* Purchase Confirmation Modal */}
      <Modal
        title={t('确认订阅')}
        visible={confirmVisible}
        onCancel={() => setConfirmVisible(false)}
        footer={null}
        width={400}
      >
        {selectedPlan && (
          <div className="space-y-4">
            <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
              <h4 className="font-semibold text-lg">{selectedPlan.name}</h4>
              <p className="text-gray-500 text-sm mt-1">{selectedPlan.description}</p>
              <div className="mt-3 flex items-baseline gap-1">
                <span className="text-2xl font-bold text-primary">
                  {selectedPlan.currency === 'CNY' ? '¥' : '$'}
                  {selectedPlan.price}
                </span>
                <span className="text-gray-500">/ {selectedPlan.days} {t('天')}</span>
              </div>
            </div>

            <div className="space-y-2">
              <p className="text-sm text-gray-500">{t('选择支付方式')}:</p>
              {getPaymentMethods().map((method) => (
                <button
                  key={method.key}
                  onClick={() => handlePurchase(method.key)}
                  disabled={purchaseLoading}
                  className="w-full py-3 px-4 rounded-lg border border-gray-200 hover:border-primary hover:bg-primary/5 transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
                >
                  <span
                    className="w-3 h-3 rounded-full"
                    style={{ backgroundColor: method.color }}
                  />
                  <span>{method.name}</span>
                </button>
              ))}
              {getPaymentMethods().length === 0 && (
                <p className="text-center text-gray-500 py-4">
                  {t('暂无可用支付方式，请联系管理员')}
                </p>
              )}
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default SubscriptionPurchase;
