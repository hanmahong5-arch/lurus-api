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
import { Spin } from '@douyinfe/semi-ui';
import SettingsHeaderNavModules from '../../pages/Setting/Operation/SettingsHeaderNavModules';
import SettingsSidebarModulesAdmin from '../../pages/Setting/Operation/SettingsSidebarModulesAdmin';
import useSettingsOptions from '../../pages/Setting/useSettingsOptions';

const INITIAL_STATE = {
  HeaderNavModules: '',
  SidebarModulesAdmin: '',
};

const UIModulesSettingPage = () => {
  const { inputs, loading, refresh } = useSettingsOptions(INITIAL_STATE);

  return (
    <Spin spinning={loading} size='large'>
      <div style={{ marginTop: '10px' }}>
        <SettingsHeaderNavModules options={inputs} refresh={refresh} />
      </div>
      <div style={{ marginTop: '10px' }}>
        <SettingsSidebarModulesAdmin options={inputs} refresh={refresh} />
      </div>
    </Spin>
  );
};

export default UIModulesSettingPage;
