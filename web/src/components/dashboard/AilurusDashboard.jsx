/*
 * AilurusDashboard - Complete Ailurus Styled Dashboard
 *
 * A beautiful dashboard with the Ailurus aesthetic:
 * - Glassmorphic panels
 * - Spring-based animations
 * - Luminous depth effects
 * - Organic texture overlays
 *
 * This component is a drop-in replacement for the original Dashboard,
 * providing the same functionality with the Ailurus design language.
 */

import React, { useContext, useEffect } from 'react';
import { motion } from 'framer-motion';
import { getRelativeTime } from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import clsx from 'clsx';

// Ailurus styled components
import AilurusDashboardHeader from './AilurusDashboardHeader';
import AilurusStatsCards from './AilurusStatsCards';
import AilurusChartsPanel from './AilurusChartsPanel';

// Original panels (to be styled incrementally)
import ApiInfoPanel from './ApiInfoPanel';
import AnnouncementsPanel from './AnnouncementsPanel';
import FaqPanel from './FaqPanel';
import UptimePanel from './UptimePanel';
import SearchModal from './modals/SearchModal';

// Hooks
import { useDashboardData } from '../../hooks/dashboard/useDashboardData';
import { useDashboardStats } from '../../hooks/dashboard/useDashboardStats';
import { useDashboardCharts } from '../../hooks/dashboard/useDashboardCharts';

// Constants and helpers
import {
  CHART_CONFIG,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  ILLUSTRATION_SIZE,
  ANNOUNCEMENT_LEGEND_DATA,
  UPTIME_STATUS_MAP,
} from '../../constants/dashboard.constants';
import {
  getTrendSpec,
  handleCopyUrl,
  handleSpeedTest,
  getUptimeStatusColor,
  getUptimeStatusText,
  renderMonitorList,
} from '../../helpers/dashboard';
import { pageVariants, staggerContainer, staggerItem, springConfig } from '../ailurus-ui/motion';

const AilurusDashboard = () => {
  // ========== Context ==========
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState, statusDispatch] = useContext(StatusContext);

  // ========== Main data management ==========
  const dashboardData = useDashboardData(userState, userDispatch, statusState);

  // ========== Chart management ==========
  const dashboardCharts = useDashboardCharts(
    dashboardData.dataExportDefaultTime,
    dashboardData.setTrendData,
    dashboardData.setConsumeQuota,
    dashboardData.setTimes,
    dashboardData.setConsumeTokens,
    dashboardData.setPieData,
    dashboardData.setLineData,
    dashboardData.setModelColors,
    dashboardData.t,
  );

  // ========== Statistics data ==========
  const { groupedStatsData } = useDashboardStats(
    userState,
    dashboardData.consumeQuota,
    dashboardData.consumeTokens,
    dashboardData.times,
    dashboardData.trendData,
    dashboardData.performanceMetrics,
    dashboardData.navigate,
    dashboardData.t,
  );

  // ========== Data processing ==========
  const initChart = async () => {
    await dashboardData.loadQuotaData().then((data) => {
      if (data && data.length > 0) {
        dashboardCharts.updateChartData(data);
      }
    });
    await dashboardData.loadUptimeData();
  };

  const handleRefresh = async () => {
    const data = await dashboardData.refresh();
    if (data && data.length > 0) {
      dashboardCharts.updateChartData(data);
    }
  };

  const handleSearchConfirm = async () => {
    await dashboardData.handleSearchConfirm(dashboardCharts.updateChartData);
  };

  // ========== Data preparation ==========
  const apiInfoData = statusState?.status?.api_info || [];
  const announcementData = (statusState?.status?.announcements || []).map(
    (item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      const relativeTime = getRelativeTime(item.publishDate);
      return {
        ...item,
        time: absoluteTime,
        relative: relativeTime,
      };
    },
  );
  const faqData = statusState?.status?.faq || [];

  const uptimeLegendData = Object.entries(UPTIME_STATUS_MAP).map(
    ([status, info]) => ({
      status: Number(status),
      color: info.color,
      label: dashboardData.t(info.label),
    }),
  );

  // ========== Effects ==========
  useEffect(() => {
    initChart();
  }, []);

  return (
    <motion.div
      className="h-full relative"
      variants={pageVariants}
      initial="initial"
      animate="animate"
      exit="exit"
    >
      {/* Background gradient overlay */}
      <div className="fixed inset-0 pointer-events-none -z-10">
        <div className="absolute top-0 left-1/4 w-96 h-96 bg-ailurus-rust-500/5 rounded-full blur-3xl" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-ailurus-teal-500/5 rounded-full blur-3xl" />
        <div className="absolute bottom-0 left-1/2 w-96 h-96 bg-ailurus-purple-500/5 rounded-full blur-3xl" />
      </div>

      {/* Dashboard Header */}
      <AilurusDashboardHeader
        getGreeting={dashboardData.getGreeting}
        greetingVisible={dashboardData.greetingVisible}
        showSearchModal={dashboardData.showSearchModal}
        refresh={handleRefresh}
        loading={dashboardData.loading}
        t={dashboardData.t}
      />

      {/* Search Modal */}
      <SearchModal
        searchModalVisible={dashboardData.searchModalVisible}
        handleSearchConfirm={handleSearchConfirm}
        handleCloseModal={dashboardData.handleCloseModal}
        isMobile={dashboardData.isMobile}
        isAdminUser={dashboardData.isAdminUser}
        inputs={dashboardData.inputs}
        dataExportDefaultTime={dashboardData.dataExportDefaultTime}
        timeOptions={dashboardData.timeOptions}
        handleInputChange={dashboardData.handleInputChange}
        t={dashboardData.t}
      />

      {/* Stats Cards */}
      <AilurusStatsCards
        groupedStatsData={groupedStatsData}
        loading={dashboardData.loading}
        getTrendSpec={getTrendSpec}
        CARD_PROPS={CARD_PROPS}
        CHART_CONFIG={CHART_CONFIG}
      />

      {/* API Info and Charts Panel */}
      <motion.div
        className="mb-6"
        variants={staggerContainer}
        initial="initial"
        animate="animate"
      >
        <div
          className={clsx(
            'grid grid-cols-1 gap-4',
            dashboardData.hasApiInfoPanel ? 'lg:grid-cols-4' : ''
          )}
        >
          <AilurusChartsPanel
            activeChartTab={dashboardData.activeChartTab}
            setActiveChartTab={dashboardData.setActiveChartTab}
            spec_line={dashboardCharts.spec_line}
            spec_model_line={dashboardCharts.spec_model_line}
            spec_pie={dashboardCharts.spec_pie}
            spec_rank_bar={dashboardCharts.spec_rank_bar}
            CARD_PROPS={CARD_PROPS}
            CHART_CONFIG={CHART_CONFIG}
            FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
            hasApiInfoPanel={dashboardData.hasApiInfoPanel}
            t={dashboardData.t}
          />

          {dashboardData.hasApiInfoPanel && (
            <motion.div variants={staggerItem}>
              <ApiInfoPanel
                apiInfoData={apiInfoData}
                handleCopyUrl={(url) => handleCopyUrl(url, dashboardData.t)}
                handleSpeedTest={handleSpeedTest}
                CARD_PROPS={CARD_PROPS}
                FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
                ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                t={dashboardData.t}
              />
            </motion.div>
          )}
        </div>
      </motion.div>

      {/* Announcements, FAQ, and Uptime Panels */}
      {dashboardData.hasInfoPanels && (
        <motion.div
          className="mb-6"
          variants={staggerContainer}
          initial="initial"
          animate="animate"
        >
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
            {/* Announcements Card */}
            {dashboardData.announcementsEnabled && (
              <motion.div variants={staggerItem}>
                <AnnouncementsPanel
                  announcementData={announcementData}
                  announcementLegendData={ANNOUNCEMENT_LEGEND_DATA.map(
                    (item) => ({
                      ...item,
                      label: dashboardData.t(item.label),
                    }),
                  )}
                  CARD_PROPS={CARD_PROPS}
                  ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                  t={dashboardData.t}
                />
              </motion.div>
            )}

            {/* FAQ Card */}
            {dashboardData.faqEnabled && (
              <motion.div variants={staggerItem}>
                <FaqPanel
                  faqData={faqData}
                  CARD_PROPS={CARD_PROPS}
                  FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
                  ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                  t={dashboardData.t}
                />
              </motion.div>
            )}

            {/* Uptime Card */}
            {dashboardData.uptimeEnabled && (
              <motion.div variants={staggerItem}>
                <UptimePanel
                  uptimeData={dashboardData.uptimeData}
                  uptimeLoading={dashboardData.uptimeLoading}
                  activeUptimeTab={dashboardData.activeUptimeTab}
                  setActiveUptimeTab={dashboardData.setActiveUptimeTab}
                  loadUptimeData={dashboardData.loadUptimeData}
                  uptimeLegendData={uptimeLegendData}
                  renderMonitorList={(monitors) =>
                    renderMonitorList(
                      monitors,
                      (status) => getUptimeStatusColor(status, UPTIME_STATUS_MAP),
                      (status) =>
                        getUptimeStatusText(
                          status,
                          UPTIME_STATUS_MAP,
                          dashboardData.t,
                        ),
                      dashboardData.t,
                    )
                  }
                  CARD_PROPS={CARD_PROPS}
                  ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                  t={dashboardData.t}
                />
              </motion.div>
            )}
          </div>
        </motion.div>
      )}
    </motion.div>
  );
};

export default AilurusDashboard;
