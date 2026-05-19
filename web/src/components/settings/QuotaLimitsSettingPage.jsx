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
import React from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsCreditLimit from '../../pages/Setting/Operation/SettingsCreditLimit';
import SettingsCheckin from '../../pages/Setting/Operation/SettingsCheckin';
import RateLimitSetting from './RateLimitSetting';
import useSettingsOptions from '../../pages/Setting/useSettingsOptions';

const INITIAL_STATE = {
  QuotaForNewUser: 0,
  PreConsumedQuota: 0,
  QuotaForInviter: 0,
  QuotaForInvitee: 0,
  'quota_setting.enable_free_model_pre_consume': true,
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,
};

const QuotaLimitsSettingPage = () => {
  const { inputs, loading, refresh } = useSettingsOptions(INITIAL_STATE);

  return (
    <Spin spinning={loading} size='large'>
      <Card style={{ marginTop: '10px' }}>
        <SettingsCreditLimit options={inputs} refresh={refresh} />
      </Card>
      <Card style={{ marginTop: '10px' }}>
        <SettingsCheckin options={inputs} refresh={refresh} />
      </Card>
      <div style={{ marginTop: '10px' }}>
        <RateLimitSetting />
      </div>
    </Spin>
  );
};

export default QuotaLimitsSettingPage;
