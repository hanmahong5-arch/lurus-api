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

/**
 * AilurusLoginForm - Ailurus-styled Login Form
 *
 * A beautiful login form implementing the Ailurus design system:
 * - Glassmorphic card with luminous shadows
 * - Spring-based animations with framer-motion
 * - Animated background blur balls
 * - Supports all OAuth providers, SMS, Passkey, and email login
 */

import React, { useContext, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { motion, AnimatePresence } from 'framer-motion';
import { UserContext } from '../../context/User';
import {
  API,
  getLogo,
  showError,
  showInfo,
  showSuccess,
  updateAPI,
  getSystemName,
  setUserData,
  onGitHubOAuthClicked,
  onDiscordOAuthClicked,
  onOIDCClicked,
  onLinuxDOOAuthClicked,
  prepareCredentialRequestOptions,
  buildAssertionResult,
  isPasskeySupported,
} from '../../helpers';
import Turnstile from 'react-turnstile';
import { Checkbox, Modal, Form } from '@douyinfe/semi-ui';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';
import TelegramLoginButton from 'react-telegram-login';
import {
  IconGithubLogo,
  IconMail,
  IconLock,
  IconKey,
  IconPhone,
  IconArrowLeft,
} from '@douyinfe/semi-icons';
import OIDCIcon from '../common/logo/OIDCIcon';
import WeChatIcon from '../common/logo/WeChatIcon';
import LinuxDoIcon from '../common/logo/LinuxDoIcon';
import TwoFAVerification from './TwoFAVerification';
import { useTranslation } from 'react-i18next';
import { SiDiscord } from 'react-icons/si';

// Import Ailurus UI components
import AilurusAuthLayout, {
  AilurusAuthDivider,
  AilurusOAuthButton,
  AilurusAuthFooter,
} from '../ailurus-ui/AilurusAuthLayout';
import AilurusButton from '../ailurus-ui/AilurusButton';
import AilurusInput from '../ailurus-ui/AilurusInput';
import { staggerContainer, staggerItem, springConfig } from '../ailurus-ui/motion';

/**
 * AilurusLoginForm - Main login form component with Ailurus styling
 */
const AilurusLoginForm = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();

  // Form state
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
    wechat_verification_code: '',
  });
  const { username, password } = inputs;
  const [searchParams] = useSearchParams();
  const [submitted, setSubmitted] = useState(false);
  const [userState, userDispatch] = useContext(UserContext);

  // Turnstile verification state
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');

  // UI state
  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);
  const [showEmailLogin, setShowEmailLogin] = useState(false);
  const [showTwoFA, setShowTwoFA] = useState(false);
  const [showSmsLogin, setShowSmsLogin] = useState(false);

  // Loading states
  const [wechatLoading, setWechatLoading] = useState(false);
  const [githubLoading, setGithubLoading] = useState(false);
  const [discordLoading, setDiscordLoading] = useState(false);
  const [oidcLoading, setOidcLoading] = useState(false);
  const [linuxdoLoading, setLinuxdoLoading] = useState(false);
  const [loginLoading, setLoginLoading] = useState(false);
  const [wechatCodeSubmitLoading, setWechatCodeSubmitLoading] = useState(false);
  const [passkeyLoading, setPasskeyLoading] = useState(false);

  // Passkey support
  const [passkeySupported, setPasskeySupported] = useState(false);

  // Terms agreement
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [hasUserAgreement, setHasUserAgreement] = useState(false);
  const [hasPrivacyPolicy, setHasPrivacyPolicy] = useState(false);

  // GitHub button state
  const [githubButtonText, setGithubButtonText] = useState('GitHub');
  const [githubButtonDisabled, setGithubButtonDisabled] = useState(false);
  const githubTimeoutRef = useRef(null);

  // SMS login states
  const [smsPhone, setSmsPhone] = useState('');
  const [smsCode, setSmsCode] = useState('');
  const [smsLoading, setSmsLoading] = useState(false);
  const [smsSendLoading, setSmsSendLoading] = useState(false);
  const [smsDisableButton, setSmsDisableButton] = useState(false);
  const [smsCountdown, setSmsCountdown] = useState(60);

  // Get system info
  const logo = getLogo();
  const systemName = getSystemName();

  // Handle affiliate code
  const affCode = new URLSearchParams(window.location.search).get('aff');
  if (affCode) {
    localStorage.setItem('aff', affCode);
  }

  // Get status from localStorage
  const [status] = useState(() => {
    const savedStatus = localStorage.getItem('status');
    return savedStatus ? JSON.parse(savedStatus) : {};
  });

  // Initialize effects
  useEffect(() => {
    if (status.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(status.turnstile_site_key);
    }
    setHasUserAgreement(status.user_agreement_enabled || false);
    setHasPrivacyPolicy(status.privacy_policy_enabled || false);
  }, [status]);

  useEffect(() => {
    isPasskeySupported()
      .then(setPasskeySupported)
      .catch(() => setPasskeySupported(false));

    return () => {
      if (githubTimeoutRef.current) {
        clearTimeout(githubTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (searchParams.get('expired')) {
      showError(t('未登录或登录已过期，请重新登录'));
    }
  }, []);

  // SMS countdown effect
  useEffect(() => {
    let countdownInterval = null;
    if (smsDisableButton && smsCountdown > 0) {
      countdownInterval = setInterval(() => {
        setSmsCountdown((prev) => prev - 1);
      }, 1000);
    } else if (smsCountdown === 0) {
      setSmsDisableButton(false);
      setSmsCountdown(60);
    }
    return () => clearInterval(countdownInterval);
  }, [smsDisableButton, smsCountdown]);

  // Check terms agreement
  const checkTermsAgreement = () => {
    if ((hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms) {
      showInfo(t('请先阅读并同意用户协议和隐私政策'));
      return false;
    }
    return true;
  };

  // Handle input change
  const handleChange = (name, value) => {
    setInputs((prev) => ({ ...prev, [name]: value }));
  };

  // Handle email/password login submit
  const handleSubmit = async (e) => {
    e?.preventDefault();
    if (!checkTermsAgreement()) return;
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setSubmitted(true);
    setLoginLoading(true);

    try {
      if (username && password) {
        const res = await API.post(
          `/api/user/login?turnstile=${turnstileToken}`,
          { username, password }
        );
        const { success, message, data } = res.data;
        if (success) {
          if (data && data.require_2fa) {
            setShowTwoFA(true);
            setLoginLoading(false);
            return;
          }
          userDispatch({ type: 'login', payload: data });
          setUserData(data);
          updateAPI();
          showSuccess(t('登录成功！'));
          if (username === 'root' && password === '123456') {
            Modal.error({
              title: t('您正在使用默认密码！'),
              content: t('请立刻修改默认密码！'),
              centered: true,
            });
          }
          navigate('/console');
        } else {
          showError(message);
        }
      } else {
        showError(t('请输入用户名和密码！'));
      }
    } catch (error) {
      showError(t('登录失败，请重试'));
    } finally {
      setLoginLoading(false);
    }
  };

  // OAuth handlers
  const handleGitHubClick = () => {
    if (!checkTermsAgreement() || githubButtonDisabled) return;
    setGithubLoading(true);
    setGithubButtonDisabled(true);
    setGithubButtonText(t('正在跳转...'));
    if (githubTimeoutRef.current) clearTimeout(githubTimeoutRef.current);
    githubTimeoutRef.current = setTimeout(() => {
      setGithubLoading(false);
      setGithubButtonText(t('请求超时'));
      setGithubButtonDisabled(true);
    }, 20000);
    try {
      onGitHubOAuthClicked(status.github_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setGithubLoading(false), 3000);
    }
  };

  const handleDiscordClick = () => {
    if (!checkTermsAgreement()) return;
    setDiscordLoading(true);
    try {
      onDiscordOAuthClicked(status.discord_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setDiscordLoading(false), 3000);
    }
  };

  const handleOIDCClick = () => {
    if (!checkTermsAgreement()) return;
    setOidcLoading(true);
    try {
      onOIDCClicked(
        status.oidc_authorization_endpoint,
        status.oidc_client_id,
        false,
        { shouldLogout: true }
      );
    } finally {
      setTimeout(() => setOidcLoading(false), 3000);
    }
  };

  const handleLinuxDOClick = () => {
    if (!checkTermsAgreement()) return;
    setLinuxdoLoading(true);
    try {
      onLinuxDOOAuthClicked(status.linuxdo_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setLinuxdoLoading(false), 3000);
    }
  };

  const handleWeChatClick = () => {
    if (!checkTermsAgreement()) return;
    setWechatLoading(true);
    setShowWeChatLoginModal(true);
    setWechatLoading(false);
  };

  const onSubmitWeChatVerificationCode = async () => {
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setWechatCodeSubmitLoading(true);
    try {
      const res = await API.get(
        `/api/oauth/wechat?code=${inputs.wechat_verification_code}`
      );
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        setUserData(data);
        updateAPI();
        navigate('/');
        showSuccess(t('登录成功！'));
        setShowWeChatLoginModal(false);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('登录失败，请重试'));
    } finally {
      setWechatCodeSubmitLoading(false);
    }
  };

  // Telegram login handler
  const onTelegramLoginClicked = async (response) => {
    if (!checkTermsAgreement()) return;
    const fields = [
      'id',
      'first_name',
      'last_name',
      'username',
      'photo_url',
      'auth_date',
      'hash',
      'lang',
    ];
    const params = {};
    fields.forEach((field) => {
      if (response[field]) params[field] = response[field];
    });
    try {
      const res = await API.get(`/api/oauth/telegram/login`, { params });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        showSuccess(t('登录成功！'));
        setUserData(data);
        updateAPI();
        navigate('/');
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('登录失败，请重试'));
    }
  };

  // Passkey login handler
  const handlePasskeyLogin = async () => {
    if (!checkTermsAgreement()) return;
    if (!passkeySupported || !window.PublicKeyCredential) {
      showInfo(t('当前环境不支持 Passkey 登录'));
      return;
    }
    setPasskeyLoading(true);
    try {
      const beginRes = await API.post('/api/user/passkey/login/begin');
      const { success, message, data } = beginRes.data;
      if (!success) {
        showError(message || t('无法发起 Passkey 登录'));
        return;
      }
      const publicKeyOptions = prepareCredentialRequestOptions(
        data?.options || data?.publicKey || data
      );
      const assertion = await navigator.credentials.get({
        publicKey: publicKeyOptions,
      });
      const payload = buildAssertionResult(assertion);
      if (!payload) {
        showError(t('Passkey 验证失败，请重试'));
        return;
      }
      const finishRes = await API.post(
        '/api/user/passkey/login/finish',
        payload
      );
      const finish = finishRes.data;
      if (finish.success) {
        userDispatch({ type: 'login', payload: finish.data });
        setUserData(finish.data);
        updateAPI();
        showSuccess(t('登录成功！'));
        navigate('/console');
      } else {
        showError(finish.message || t('Passkey 登录失败，请重试'));
      }
    } catch (error) {
      if (error?.name === 'AbortError') {
        showInfo(t('已取消 Passkey 登录'));
      } else {
        showError(t('Passkey 登录失败，请重试'));
      }
    } finally {
      setPasskeyLoading(false);
    }
  };

  // SMS login handlers
  const handleSmsLoginClick = () => {
    if (!checkTermsAgreement()) return;
    setShowSmsLogin(true);
    setShowEmailLogin(false);
  };

  const sendSmsCode = async () => {
    if (!smsPhone) {
      showInfo(t('请输入手机号'));
      return;
    }
    const phoneRegex = /^1[3-9]\d{9}$/;
    if (!phoneRegex.test(smsPhone)) {
      showInfo(t('请输入正确的手机号格式'));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setSmsSendLoading(true);
    try {
      const res = await API.post(`/api/sms/send?turnstile=${turnstileToken}`, {
        phone: smsPhone,
        purpose: 'login',
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('验证码发送成功'));
        setSmsDisableButton(true);
      } else {
        showError(message || t('发送验证码失败'));
      }
    } catch (error) {
      showError(t('发送验证码失败，请重试'));
    } finally {
      setSmsSendLoading(false);
    }
  };

  const handleSmsLogin = async () => {
    if (!checkTermsAgreement()) return;
    if (!smsPhone || !smsCode) {
      showInfo(t('请输入手机号和验证码'));
      return;
    }
    setSmsLoading(true);
    try {
      const res = await API.post('/api/user/login_sms', {
        phone: smsPhone,
        code: smsCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        setUserData(data);
        updateAPI();
        showSuccess(t('登录成功！'));
        navigate('/console');
      } else {
        showError(message || t('登录失败'));
      }
    } catch (error) {
      showError(t('登录失败，请重试'));
    } finally {
      setSmsLoading(false);
    }
  };

  // Navigation handlers
  const handleBackToOtherOptions = () => {
    setShowSmsLogin(false);
    setShowEmailLogin(false);
  };

  const handleEmailLoginClick = () => {
    setShowEmailLogin(true);
    setShowSmsLogin(false);
  };

  // 2FA handlers
  const handle2FASuccess = (data) => {
    userDispatch({ type: 'login', payload: data });
    setUserData(data);
    updateAPI();
    showSuccess(t('登录成功！'));
    navigate('/console');
  };

  const handleBackToLogin = () => {
    setShowTwoFA(false);
    setInputs({ username: '', password: '', wechat_verification_code: '' });
  };

  // Check if any OAuth is enabled
  const hasOAuthOptions =
    status.github_oauth ||
    status.discord_oauth ||
    status.oidc_enabled ||
    status.wechat_login ||
    status.linuxdo_oauth ||
    status.telegram_oauth ||
    status.sms_enabled;

  // Terms checkbox component
  const TermsCheckbox = () => {
    if (!hasUserAgreement && !hasPrivacyPolicy) return null;
    return (
      <motion.div
        className="mt-6"
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.3 }}
      >
        <Checkbox
          checked={agreedToTerms}
          onChange={(e) => setAgreedToTerms(e.target.checked)}
          className="ailurus-checkbox"
        >
          <Text size="small" className="text-gray-600 dark:text-ailurus-cream/70">
            {t('我已阅读并同意')}
            {hasUserAgreement && (
              <a
                href="/user-agreement"
                target="_blank"
                rel="noopener noreferrer"
                className="text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-700 dark:hover:text-ailurus-rust-300 mx-1 transition-colors"
              >
                {t('用户协议')}
              </a>
            )}
            {hasUserAgreement && hasPrivacyPolicy && t('和')}
            {hasPrivacyPolicy && (
              <a
                href="/privacy-policy"
                target="_blank"
                rel="noopener noreferrer"
                className="text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-700 dark:hover:text-ailurus-rust-300 mx-1 transition-colors"
              >
                {t('隐私政策')}
              </a>
            )}
          </Text>
        </Checkbox>
      </motion.div>
    );
  };

  // Render OAuth options (main login screen)
  const renderOAuthOptions = () => {
    return (
      <motion.div
        className="space-y-4"
        variants={staggerContainer}
        initial="initial"
        animate="animate"
      >
        {/* OAuth Buttons */}
        <div className="space-y-3">
          {status.wechat_login && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={<WeChatIcon style={{ color: '#07C160' }} />}
                provider={t('微信')}
                onClick={handleWeChatClick}
                loading={wechatLoading}
              />
            </motion.div>
          )}

          {status.github_oauth && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={<IconGithubLogo size="large" />}
                provider={githubButtonText}
                onClick={handleGitHubClick}
                loading={githubLoading}
                disabled={githubButtonDisabled}
              />
            </motion.div>
          )}

          {status.discord_oauth && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={
                  <SiDiscord
                    style={{ color: '#5865F2', width: '20px', height: '20px' }}
                  />
                }
                provider="Discord"
                onClick={handleDiscordClick}
                loading={discordLoading}
              />
            </motion.div>
          )}

          {status.oidc_enabled && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={<OIDCIcon style={{ color: '#1877F2' }} />}
                provider="OIDC"
                onClick={handleOIDCClick}
                loading={oidcLoading}
              />
            </motion.div>
          )}

          {status.linuxdo_oauth && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={
                  <LinuxDoIcon
                    style={{ color: '#E95420', width: '20px', height: '20px' }}
                  />
                }
                provider="LinuxDO"
                onClick={handleLinuxDOClick}
                loading={linuxdoLoading}
              />
            </motion.div>
          )}

          {status.telegram_oauth && (
            <motion.div variants={staggerItem} className="flex justify-center my-2">
              <TelegramLoginButton
                dataOnauth={onTelegramLoginClicked}
                botName={status.telegram_bot_name}
              />
            </motion.div>
          )}

          {status.sms_enabled && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={<IconPhone size="large" />}
                provider={t('手机号')}
                onClick={handleSmsLoginClick}
              />
            </motion.div>
          )}

          {status.passkey_login && passkeySupported && (
            <motion.div variants={staggerItem}>
              <AilurusOAuthButton
                icon={<IconKey size="large" />}
                provider="Passkey"
                onClick={handlePasskeyLogin}
                loading={passkeyLoading}
              />
            </motion.div>
          )}
        </div>

        <AilurusAuthDivider text={t('或')} />

        {/* Email login button */}
        <motion.div variants={staggerItem}>
          <AilurusButton
            variant="primary"
            fullWidth
            leftIcon={<IconMail size="large" />}
            onClick={handleEmailLoginClick}
          >
            {t('使用 邮箱或用户名 登录')}
          </AilurusButton>
        </motion.div>

        <TermsCheckbox />
      </motion.div>
    );
  };

  // Render email login form
  const renderEmailLoginForm = () => {
    return (
      <motion.div
        className="space-y-6"
        variants={staggerContainer}
        initial="initial"
        animate="animate"
      >
        {/* Back button if OAuth options exist */}
        {hasOAuthOptions && (
          <motion.button
            variants={staggerItem}
            className="flex items-center text-gray-500 dark:text-ailurus-cream/60 hover:text-gray-700 dark:hover:text-ailurus-cream transition-colors"
            onClick={handleBackToOtherOptions}
            whileHover={{ x: -4 }}
            transition={springConfig.snappy}
          >
            <IconArrowLeft size="small" className="mr-2" />
            {t('其他登录选项')}
          </motion.button>
        )}

        {/* Passkey button if supported */}
        {status.passkey_login && passkeySupported && (
          <motion.div variants={staggerItem}>
            <AilurusOAuthButton
              icon={<IconKey size="large" />}
              provider="Passkey"
              onClick={handlePasskeyLogin}
              loading={passkeyLoading}
            />
          </motion.div>
        )}

        {/* Login form */}
        <form onSubmit={handleSubmit} className="space-y-5">
          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('用户名或邮箱')}
              placeholder={t('请输入您的用户名或邮箱地址')}
              value={username}
              onChange={(e) => handleChange('username', e.target.value)}
              leftIcon={<IconMail />}
              size="md"
            />
          </motion.div>

          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('密码')}
              placeholder={t('请输入您的密码')}
              type="password"
              value={password}
              onChange={(e) => handleChange('password', e.target.value)}
              leftIcon={<IconLock />}
              size="md"
            />
          </motion.div>

          <TermsCheckbox />

          <motion.div variants={staggerItem} className="space-y-3 pt-2">
            <AilurusButton
              variant="primary"
              fullWidth
              type="submit"
              loading={loginLoading}
              disabled={
                (hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms
              }
            >
              {t('继续')}
            </AilurusButton>

            <AilurusButton
              variant="ghost"
              fullWidth
              onClick={() => navigate('/reset')}
            >
              {t('忘记密码？')}
            </AilurusButton>
          </motion.div>
        </form>
      </motion.div>
    );
  };

  // Render SMS login form
  const renderSmsLoginForm = () => {
    return (
      <motion.div
        className="space-y-6"
        variants={staggerContainer}
        initial="initial"
        animate="animate"
      >
        {/* Back button */}
        <motion.button
          variants={staggerItem}
          className="flex items-center text-gray-500 dark:text-ailurus-cream/60 hover:text-gray-700 dark:hover:text-ailurus-cream transition-colors"
          onClick={handleBackToOtherOptions}
          whileHover={{ x: -4 }}
          transition={springConfig.snappy}
        >
          <IconArrowLeft size="small" className="mr-2" />
          {t('其他登录选项')}
        </motion.button>

        {/* SMS form */}
        <form onSubmit={(e) => { e.preventDefault(); handleSmsLogin(); }} className="space-y-5">
          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('手机号')}
              placeholder={t('请输入您的手机号')}
              value={smsPhone}
              onChange={(e) => setSmsPhone(e.target.value)}
              leftIcon={<IconPhone />}
              size="md"
            />
          </motion.div>

          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('验证码')}
              placeholder={t('请输入验证码')}
              value={smsCode}
              onChange={(e) => setSmsCode(e.target.value)}
              leftIcon={<IconKey />}
              size="md"
              rightIcon={
                <button
                  type="button"
                  onClick={sendSmsCode}
                  disabled={smsDisableButton || smsSendLoading}
                  className="text-xs text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-700 dark:hover:text-ailurus-rust-300 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap font-medium"
                >
                  {smsDisableButton
                    ? `${smsCountdown}s`
                    : t('获取验证码')}
                </button>
              }
            />
          </motion.div>

          <TermsCheckbox />

          <motion.div variants={staggerItem} className="pt-2">
            <AilurusButton
              variant="primary"
              fullWidth
              type="submit"
              loading={smsLoading}
              disabled={
                (hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms
              }
            >
              {t('登录')}
            </AilurusButton>
          </motion.div>
        </form>
      </motion.div>
    );
  };

  // WeChat modal
  const renderWeChatModal = () => {
    return (
      <Modal
        title={t('微信扫码登录')}
        visible={showWeChatLoginModal}
        maskClosable={true}
        onOk={onSubmitWeChatVerificationCode}
        onCancel={() => setShowWeChatLoginModal(false)}
        okText={t('登录')}
        centered={true}
        okButtonProps={{ loading: wechatCodeSubmitLoading }}
      >
        <div className="flex flex-col items-center">
          <img src={status.wechat_qrcode} alt="微信二维码" className="mb-4" />
        </div>
        <div className="text-center mb-4">
          <p>{t('微信扫码关注公众号，输入「验证码」获取验证码（三分钟内有效）')}</p>
        </div>
        <Form>
          <Form.Input
            field="wechat_verification_code"
            placeholder={t('验证码')}
            label={t('验证码')}
            value={inputs.wechat_verification_code}
            onChange={(value) => handleChange('wechat_verification_code', value)}
          />
        </Form>
      </Modal>
    );
  };

  // 2FA modal
  const render2FAModal = () => {
    return (
      <Modal
        title={
          <div className="flex items-center">
            <div className="w-8 h-8 rounded-full bg-ailurus-teal-500/20 flex items-center justify-center mr-3">
              <IconKey className="text-ailurus-teal-400" />
            </div>
            {t('两步验证')}
          </div>
        }
        visible={showTwoFA}
        onCancel={handleBackToLogin}
        footer={null}
        width={450}
        centered
      >
        <TwoFAVerification
          onSuccess={handle2FASuccess}
          onBack={handleBackToLogin}
          isModal={true}
        />
      </Modal>
    );
  };

  // Determine which view to show
  const getLoginTitle = () => {
    if (showSmsLogin) return t('手机号登录');
    return t('登 录');
  };

  const getLoginSubtitle = () => {
    if (showEmailLogin) return t('使用您的账户继续');
    if (showSmsLogin) return t('输入您的手机号接收验证码');
    return t('选择一种方式登录您的账户');
  };

  return (
    <AilurusAuthLayout
      logo={logo}
      systemName={systemName}
      title={getLoginTitle()}
      subtitle={getLoginSubtitle()}
    >
      {/* Main content */}
      <AnimatePresence mode="wait">
        {showSmsLogin ? (
          <motion.div
            key="sms"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={springConfig.snappy}
          >
            {renderSmsLoginForm()}
          </motion.div>
        ) : showEmailLogin || !hasOAuthOptions ? (
          <motion.div
            key="email"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={springConfig.snappy}
          >
            {renderEmailLoginForm()}
          </motion.div>
        ) : (
          <motion.div
            key="oauth"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={springConfig.snappy}
          >
            {renderOAuthOptions()}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Register link */}
      {!status.self_use_mode_enabled && (
        <AilurusAuthFooter>
          <span className="text-gray-500 dark:text-ailurus-cream/60">{t('没有账户？')}</span>{' '}
          <Link
            to="/register"
            className="text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-700 dark:hover:text-ailurus-rust-300 font-medium transition-colors"
          >
            {t('注册')}
          </Link>
        </AilurusAuthFooter>
      )}

      {/* Turnstile verification */}
      {turnstileEnabled && (
        <div className="flex justify-center mt-6">
          <Turnstile
            sitekey={turnstileSiteKey}
            onVerify={(token) => setTurnstileToken(token)}
          />
        </div>
      )}

      {/* Modals */}
      {renderWeChatModal()}
      {render2FAModal()}
    </AilurusAuthLayout>
  );
};

export default AilurusLoginForm;
