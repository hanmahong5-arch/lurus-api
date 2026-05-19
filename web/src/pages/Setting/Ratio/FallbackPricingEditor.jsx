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
import React, { useState, useEffect, useMemo } from 'react';
import {
  Button,
  InputNumber,
  Slider,
  Table,
  Typography,
  Space,
  Tag,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

const { Text } = Typography;

const familyPreviews = [
  { family: 'gemini-flash-lite', baseRatio: 0.05 },
  { family: 'gemini-flash', baseRatio: 0.075 },
  { family: 'gemini-pro', baseRatio: 0.625 },
  { family: 'claude-haiku', baseRatio: 0.5 },
  { family: 'claude-sonnet', baseRatio: 1.5 },
  { family: 'claude-opus', baseRatio: 7.5 },
  { family: 'gpt-mini', baseRatio: 0.2 },
  { family: 'gpt (default)', baseRatio: 1.25 },
  { family: 'deepseek-chat', baseRatio: 0.07 },
  { family: 'deepseek-reasoner', baseRatio: 0.275 },
  { family: 'o3-mini', baseRatio: 0.55 },
  { family: 'qwen-turbo', baseRatio: 0.86 },
];

const FallbackPricingEditor = ({ options, refresh }) => {
  const { t } = useTranslation();
  const [markup, setMarkup] = useState(1.25);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (options?.ModelFallbackMarkup) {
      const val = parseFloat(options.ModelFallbackMarkup);
      if (!isNaN(val) && val > 0) {
        setMarkup(val);
      }
    }
  }, [options]);

  const previewData = useMemo(() => {
    return familyPreviews.map((f, idx) => ({
      key: idx,
      family: f.family,
      baseRatio: f.baseRatio,
      finalRatio: parseFloat((f.baseRatio * markup).toFixed(4)),
    }));
  }, [markup]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const res = await API.put('/api/option/', {
        key: 'ModelFallbackMarkup',
        value: String(markup),
      });
      if (res.data.success) {
        showSuccess(t('保存成功'));
        refresh?.();
      } else {
        showError(res.data.message || t('保存失败'));
      }
    } catch (_) {
      showError(t('保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const columns = [
    { title: t('模型家族'), dataIndex: 'family', width: 200 },
    {
      title: t('基础倍率'),
      dataIndex: 'baseRatio',
      width: 120,
      align: 'right',
    },
    {
      title: t('最终倍率'),
      dataIndex: 'finalRatio',
      width: 120,
      align: 'right',
      render: (val) => <Text strong>{val}</Text>,
    },
  ];

  return (
    <div style={{ padding: '16px 0' }}>
      <div style={{ marginBottom: 16 }}>
        <Text>
          {t(
            '当模型没有显式配置倍率时，系统根据模型家族自动定价，并应用利润率加成。',
          )}
        </Text>
      </div>

      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 16,
          marginBottom: 24,
        }}
      >
        <Text strong>{t('利润率加成')}:</Text>
        <Slider
          value={markup}
          min={1.0}
          max={3.0}
          step={0.05}
          style={{ width: 300 }}
          onChange={(val) => setMarkup(val)}
          tipFormatter={(val) => `${((val - 1) * 100).toFixed(0)}%`}
        />
        <InputNumber
          value={markup}
          min={1.0}
          max={3.0}
          step={0.05}
          style={{ width: 100 }}
          onChange={(val) => val && setMarkup(val)}
        />
        <Tag color='blue' size='large'>
          {((markup - 1) * 100).toFixed(0)}% {t('利润')}
        </Tag>
      </div>

      <Table
        columns={columns}
        dataSource={previewData}
        pagination={false}
        size='small'
        style={{ marginBottom: 16 }}
      />

      <Space>
        <Button
          theme='solid'
          type='primary'
          onClick={handleSave}
          loading={saving}
        >
          {t('保存')}
        </Button>
      </Space>
    </div>
  );
};

export default FallbackPricingEditor;
