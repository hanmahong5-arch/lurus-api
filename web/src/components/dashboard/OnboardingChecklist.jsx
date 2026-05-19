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
import React, { useState, useEffect, useCallback } from 'react';
import { Card, Steps, Button, Typography, Banner } from '@douyinfe/semi-ui';
import { IconTickCircle, IconArrowRight } from '@douyinfe/semi-icons';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API, isAdmin } from '../../helpers';

const { Text } = Typography;

const DISMISSED_KEY = 'onboarding_dismissed';

const OnboardingChecklist = ({ serverAddress }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [dismissed, setDismissed] = useState(
    () => localStorage.getItem(DISMISSED_KEY) === 'true',
  );
  const [checks, setChecks] = useState({
    hasChannels: null,
    hasTokens: null,
  });
  const [loading, setLoading] = useState(true);

  const runChecks = useCallback(async () => {
    try {
      const [chRes, tkRes] = await Promise.all([
        API.get('/api/channel/?p=0&size=1'),
        API.get('/api/token/?p=0&size=1'),
      ]);
      setChecks({
        hasChannels: (chRes?.data?.data?.length ?? 0) > 0,
        hasTokens: (tkRes?.data?.data?.length ?? 0) > 0,
      });
    } catch {
      setChecks({ hasChannels: false, hasTokens: false });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!dismissed && isAdmin()) {
      runChecks();
    } else {
      setLoading(false);
    }
  }, [dismissed, runChecks]);

  if (dismissed || loading || !isAdmin()) return null;

  const allDone = checks.hasChannels && checks.hasTokens;
  if (allDone) return null;

  const steps = [
    {
      key: 'channel',
      title: t('onboarding_step_channel'),
      description: t('onboarding_step_channel_desc'),
      done: checks.hasChannels,
      action: () => navigate('/console/channel'),
    },
    {
      key: 'token',
      title: t('onboarding_step_token'),
      description: t('onboarding_step_token_desc'),
      done: checks.hasTokens,
      action: () => navigate('/console/token'),
    },
    {
      key: 'test',
      title: t('onboarding_step_test'),
      description: t('onboarding_step_test_desc'),
      done: false,
      action: null,
    },
  ];

  const currentStep = steps.findIndex((s) => !s.done);

  const handleDismiss = () => {
    localStorage.setItem(DISMISSED_KEY, 'true');
    setDismissed(true);
  };

  const curlExample = `curl ${serverAddress || window.location.origin}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'`;

  return (
    <Card
      className='mb-4'
      bodyStyle={{ padding: '16px 20px' }}
      style={{
        border: '1px solid var(--semi-color-primary-light-default)',
        background: 'var(--semi-color-primary-light-default)',
      }}
    >
      <div className='flex items-center justify-between mb-3'>
        <Text strong className='text-base'>
          {t('onboarding_title')}
        </Text>
        <Button size='small' type='tertiary' onClick={handleDismiss}>
          {t('onboarding_dismiss')}
        </Button>
      </div>

      <Steps current={currentStep} size='small' className='mb-3'>
        {steps.map((step) => (
          <Steps.Step
            key={step.key}
            title={step.title}
            description={step.description}
            icon={
              step.done ? (
                <IconTickCircle
                  style={{ color: 'var(--semi-color-success)' }}
                />
              ) : undefined
            }
          />
        ))}
      </Steps>

      {currentStep >= 0 && currentStep < 2 && steps[currentStep].action && (
        <Button
          theme='solid'
          size='small'
          icon={<IconArrowRight />}
          iconPosition='right'
          onClick={steps[currentStep].action}
        >
          {steps[currentStep].title}
        </Button>
      )}

      {currentStep === 2 && (
        <Banner
          type='info'
          closeIcon={null}
          className='rounded-lg mt-1'
          description={
            <pre className='text-xs whitespace-pre-wrap break-all m-0 font-mono'>
              {curlExample}
            </pre>
          }
        />
      )}
    </Card>
  );
};

export default OnboardingChecklist;
