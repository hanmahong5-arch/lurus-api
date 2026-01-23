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
 * AilurusRegisterForm - Ailurus-styled Registration Form
 *
 * A beautiful registration form implementing the Ailurus design system:
 * - Glassmorphic card with luminous shadows
 * - Spring-based animations with framer-motion
 * - Animated background blur balls
 * - Supports all OAuth providers, SMS, and email registration
 */

import React, { useContext, useEffect, useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
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
} from '../../helpers';
import Turnstile from 'react-turnstile';
import { Checkbox, Modal, Form } from '@douyinfe/semi-ui';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';
import TelegramLoginButton from 'react-telegram-login/src';
import {
  IconGithubLogo,
  IconMail,
  IconUser,
  IconLock,
  IconKey,
  IconPhone,
  IconArrowLeft,
} from '@douyinfe/semi-icons';
import OIDCIcon from '../common/logo/OIDCIcon';
import WeChatIcon from '../common/logo/WeChatIcon';
import LinuxDoIcon from '../common/logo/LinuxDoIcon';
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
 * AilurusRegisterForm - Main registration form component with Ailurus styling
 */
const AilurusRegisterForm = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();

  // Form state
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
    password2: '',
    email: '',
    verification_code: '',
    wechat_verification_code: '',
  });
  const { username, password, password2 } = inputs;
  const [userState, userDispatch] = useContext(UserContext);

  // Turnstile verification state
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');

  // UI state
  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);
  const [showEmailRegister, setShowEmailRegister] = useState(false);
  const [showSmsRegister, setShowSmsRegister] = useState(false);

  // Loading states
  const [wechatLoading, setWechatLoading] = useState(false);
  const [githubLoading, setGithubLoading] = useState(false);
  const [discordLoading, setDiscordLoading] = useState(false);
  const [oidcLoading, setOidcLoading] = useState(false);
  const [linuxdoLoading, setLinuxdoLoading] = useState(false);
  const [registerLoading, setRegisterLoading] = useState(false);
  const [verificationCodeLoading, setVerificationCodeLoading] = useState(false);
  const [wechatCodeSubmitLoading, setWechatCodeSubmitLoading] = useState(false);

  // Email verification countdown
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);

  // Terms agreement
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [hasUserAgreement, setHasUserAgreement] = useState(false);
  const [hasPrivacyPolicy, setHasPrivacyPolicy] = useState(false);

  // GitHub button state
  const [githubButtonText, setGithubButtonText] = useState('GitHub');
  const [githubButtonDisabled, setGithubButtonDisabled] = useState(false);
  const githubTimeoutRef = useRef(null);

  // SMS registration states
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
  let affCode = new URLSearchParams(window.location.search).get('aff');
  if (affCode) {
    localStorage.setItem('aff', affCode);
  }

  // Get status from localStorage
  const [status] = useState(() => {
    const savedStatus = localStorage.getItem('status');
    return savedStatus ? JSON.parse(savedStatus) : {};
  });

  // Email verification setting
  const [showEmailVerification, setShowEmailVerification] = useState(() => {
    return status.email_verification ?? false;
  });

  // Initialize effects
  useEffect(() => {
    setShowEmailVerification(status.email_verification);
    if (status.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(status.turnstile_site_key);
    }
    setHasUserAgreement(status.user_agreement_enabled || false);
    setHasPrivacyPolicy(status.privacy_policy_enabled || false);
  }, [status]);

  // Email verification countdown
  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown((prev) => prev - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(countdownInterval);
  }, [disableButton, countdown]);

  // Cleanup GitHub timeout
  useEffect(() => {
    return () => {
      if (githubTimeoutRef.current) {
        clearTimeout(githubTimeoutRef.current);
      }
    };
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

  // Handle email/password registration submit
  const handleSubmit = async (e) => {
    e?.preventDefault();
    if (password.length < 8) {
      showInfo(t('密码长度不得小于 8 位！'));
      return;
    }
    if (password !== password2) {
      showInfo(t('两次输入的密码不一致'));
      return;
    }
    if (!checkTermsAgreement()) return;
    if (username && password) {
      if (turnstileEnabled && turnstileToken === '') {
        showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
        return;
      }
      setRegisterLoading(true);
      try {
        if (!affCode) {
          affCode = localStorage.getItem('aff');
        }
        inputs.aff_code = affCode;
        const res = await API.post(
          `/api/user/register?turnstile=${turnstileToken}`,
          inputs
        );
        const { success, message } = res.data;
        if (success) {
          navigate('/login');
          showSuccess(t('注册成功！'));
        } else {
          showError(message);
        }
      } catch (error) {
        showError(t('注册失败，请重试'));
      } finally {
        setRegisterLoading(false);
      }
    }
  };

  // Send email verification code
  const sendVerificationCode = async () => {
    if (inputs.email === '') return;
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setVerificationCodeLoading(true);
    try {
      const res = await API.get(
        `/api/verification?email=${inputs.email}&turnstile=${turnstileToken}`
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('验证码发送成功，请检查你的邮箱！'));
        setDisableButton(true);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('发送验证码失败，请重试'));
    } finally {
      setVerificationCodeLoading(false);
    }
  };

  // OAuth handlers
  const handleGitHubClick = () => {
    if (githubButtonDisabled) return;
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
    setDiscordLoading(true);
    try {
      onDiscordOAuthClicked(status.discord_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setDiscordLoading(false), 3000);
    }
  };

  const handleOIDCClick = () => {
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
    setLinuxdoLoading(true);
    try {
      onLinuxDOOAuthClicked(status.linuxdo_client_id, { shouldLogout: true });
    } finally {
      setTimeout(() => setLinuxdoLoading(false), 3000);
    }
  };

  const handleWeChatClick = () => {
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

  // SMS registration handlers
  const handleSmsRegisterClick = () => {
    setShowSmsRegister(true);
    setShowEmailRegister(false);
  };

  const sendSmsCodeForRegister = async () => {
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

  const handleSmsRegister = async () => {
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
        showSuccess(t('注册成功！'));
        navigate('/console');
      } else {
        showError(message || t('注册失败'));
      }
    } catch (error) {
      showError(t('注册失败，请重试'));
    } finally {
      setSmsLoading(false);
    }
  };

  // Navigation handlers
  const handleBackToOtherOptions = () => {
    setShowSmsRegister(false);
    setShowEmailRegister(false);
  };

  const handleEmailRegisterClick = () => {
    setShowEmailRegister(true);
    setShowSmsRegister(false);
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
                className="text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-500 dark:hover:text-ailurus-rust-300 mx-1 transition-colors"
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
                className="text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-500 dark:hover:text-ailurus-rust-300 mx-1 transition-colors"
              >
                {t('隐私政策')}
              </a>
            )}
          </Text>
        </Checkbox>
      </motion.div>
    );
  };

  // Render OAuth options (main registration screen)
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
                onClick={handleSmsRegisterClick}
              />
            </motion.div>
          )}
        </div>

        <AilurusAuthDivider text={t('或')} />

        {/* Email registration button */}
        <motion.div variants={staggerItem}>
          <AilurusButton
            variant="primary"
            fullWidth
            leftIcon={<IconUser size="large" />}
            onClick={handleEmailRegisterClick}
          >
            {t('使用 用户名 注册')}
          </AilurusButton>
        </motion.div>
      </motion.div>
    );
  };

  // Render email registration form
  const renderEmailRegisterForm = () => {
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
            className="flex items-center text-gray-500 dark:text-ailurus-cream/60 hover:text-gray-900 dark:hover:text-ailurus-cream transition-colors"
            onClick={handleBackToOtherOptions}
            whileHover={{ x: -4 }}
            transition={springConfig.snappy}
          >
            <IconArrowLeft size="small" className="mr-2" />
            {t('其他注册选项')}
          </motion.button>
        )}

        {/* Registration form */}
        <form onSubmit={handleSubmit} className="space-y-5">
          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('用户名')}
              placeholder={t('请输入用户名')}
              value={username}
              onChange={(e) => handleChange('username', e.target.value)}
              leftIcon={<IconUser />}
              size="md"
            />
          </motion.div>

          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('密码')}
              placeholder={t('输入密码，最短 8 位，最长 20 位')}
              type="password"
              value={password}
              onChange={(e) => handleChange('password', e.target.value)}
              leftIcon={<IconLock />}
              size="md"
            />
          </motion.div>

          <motion.div variants={staggerItem}>
            <AilurusInput
              label={t('确认密码')}
              placeholder={t('确认密码')}
              type="password"
              value={password2}
              onChange={(e) => handleChange('password2', e.target.value)}
              leftIcon={<IconLock />}
              size="md"
            />
          </motion.div>

          {showEmailVerification && (
            <>
              <motion.div variants={staggerItem}>
                <AilurusInput
                  label={t('邮箱')}
                  placeholder={t('输入邮箱地址')}
                  type="email"
                  value={inputs.email}
                  onChange={(e) => handleChange('email', e.target.value)}
                  leftIcon={<IconMail />}
                  size="md"
                  rightIcon={
                    <button
                      type="button"
                      onClick={sendVerificationCode}
                      disabled={disableButton || verificationCodeLoading}
                      className="text-xs text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-500 dark:hover:text-ailurus-rust-300 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                    >
                      {disableButton
                        ? `${countdown}s`
                        : t('获取验证码')}
                    </button>
                  }
                />
              </motion.div>

              <motion.div variants={staggerItem}>
                <AilurusInput
                  label={t('验证码')}
                  placeholder={t('输入验证码')}
                  value={inputs.verification_code}
                  onChange={(e) => handleChange('verification_code', e.target.value)}
                  leftIcon={<IconKey />}
                  size="md"
                />
              </motion.div>
            </>
          )}

          <TermsCheckbox />

          <motion.div variants={staggerItem} className="pt-2">
            <AilurusButton
              variant="primary"
              fullWidth
              type="submit"
              loading={registerLoading}
              disabled={
                (hasUserAgreement || hasPrivacyPolicy) && !agreedToTerms
              }
            >
              {t('注册')}
            </AilurusButton>
          </motion.div>
        </form>
      </motion.div>
    );
  };

  // Render SMS registration form
  const renderSmsRegisterForm = () => {
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
          className="flex items-center text-gray-500 dark:text-ailurus-cream/60 hover:text-gray-900 dark:hover:text-ailurus-cream transition-colors"
          onClick={handleBackToOtherOptions}
          whileHover={{ x: -4 }}
          transition={springConfig.snappy}
        >
          <IconArrowLeft size="small" className="mr-2" />
          {t('其他注册选项')}
        </motion.button>

        {/* SMS form */}
        <form onSubmit={(e) => { e.preventDefault(); handleSmsRegister(); }} className="space-y-5">
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
                  onClick={sendSmsCodeForRegister}
                  disabled={smsDisableButton || smsSendLoading}
                  className="text-xs text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-500 dark:hover:text-ailurus-rust-300 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
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
              {t('注册')}
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

  // Determine which view to show
  const getRegisterTitle = () => {
    if (showSmsRegister) return t('手机号注册');
    return t('注 册');
  };

  const getRegisterSubtitle = () => {
    if (showEmailRegister) return t('创建您的账户');
    if (showSmsRegister) return t('输入您的手机号接收验证码');
    return t('选择一种方式创建账户');
  };

  return (
    <AilurusAuthLayout
      logo={logo}
      systemName={systemName}
      title={getRegisterTitle()}
      subtitle={getRegisterSubtitle()}
    >
      {/* Main content */}
      <AnimatePresence mode="wait">
        {showSmsRegister ? (
          <motion.div
            key="sms"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={springConfig.snappy}
          >
            {renderSmsRegisterForm()}
          </motion.div>
        ) : showEmailRegister || !hasOAuthOptions ? (
          <motion.div
            key="email"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            transition={springConfig.snappy}
          >
            {renderEmailRegisterForm()}
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

      {/* Login link */}
      <AilurusAuthFooter>
        <span className="text-gray-500 dark:text-ailurus-cream/60">{t('已有账户？')}</span>{' '}
        <Link
          to="/login"
          className="text-ailurus-rust-600 dark:text-ailurus-rust-400 hover:text-ailurus-rust-500 dark:hover:text-ailurus-rust-300 font-medium transition-colors"
        >
          {t('登录')}
        </Link>
      </AilurusAuthFooter>

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
    </AilurusAuthLayout>
  );
};

export default AilurusRegisterForm;
