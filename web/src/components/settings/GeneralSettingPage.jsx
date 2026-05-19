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
import SettingsGeneral from '../../pages/Setting/Operation/SettingsGeneral';
import useSettingsOptions from '../../pages/Setting/useSettingsOptions';

const INITIAL_STATE = {
  QuotaForNewUser: 0,
  PreConsumedQuota: 0,
  QuotaForInviter: 0,
  QuotaForInvitee: 0,
  'quota_setting.enable_free_model_pre_consume': true,
  TopUpLink: '',
  'general_setting.docs_link': '',
  QuotaPerUnit: 0,
  USDExchangeRate: 0,
  RetryTimes: 0,
  'general_setting.quota_display_type': 'USD',
  DisplayTokenStatEnabled: false,
  DefaultCollapseSidebar: false,
  DemoSiteEnabled: false,
  SelfUseModeEnabled: false,
};

const GeneralSettingPage = () => {
  const { inputs, loading, refresh } = useSettingsOptions(INITIAL_STATE);

  return (
    <Spin spinning={loading} size='large'>
      <Card style={{ marginTop: '10px' }}>
        <SettingsGeneral options={inputs} refresh={refresh} />
      </Card>
    </Spin>
  );
};

export default GeneralSettingPage;
