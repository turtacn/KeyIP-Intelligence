import React, { useState, useEffect } from 'react';
import { type ApiMode, getApiMode, setApiMode } from '../../utils/apiMode';

const MODE_NEXT: Record<ApiMode, ApiMode> = {
  mock: 'proxy',
  proxy: 'live',
  live: 'mock',
};

const MODE_MESSAGES: Record<ApiMode, string | null> = {
  mock: 'Mock 模式 — 数据为模拟数据',
  proxy: '代理模式 — 连接到 localhost:8080',
  live: null,
};

const EnvironmentBanner: React.FC = () => {
  const currentMode = getApiMode();
  const message = MODE_MESSAGES[currentMode];
  const [shouldFade, setShouldFade] = useState(false);
  const [isHovered, setIsHovered] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setShouldFade(true), 5000);
    return () => clearTimeout(timer);
  }, []);

  // Don't render for live / production mode
  if (message === null) return null;

  const show = !shouldFade || isHovered;

  const handleClick = () => {
    const next = MODE_NEXT[currentMode];
    setApiMode(next);
  };

  return (
    <div
      onClick={handleClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      className="fixed bottom-0 left-0 right-0 z-50 px-4 py-1 text-center text-xs cursor-pointer select-none transition-opacity duration-500"
      style={{
        backgroundColor: 'rgba(148, 163, 184, 0.12)',
        color: '#94a3b8',
        backdropFilter: 'blur(4px)',
        WebkitBackdropFilter: 'blur(4px)',
        opacity: show ? 1 : 0,
      }}
    >
      {message}
      <span className="ml-1 opacity-40">— 点击切换</span>
    </div>
  );
};

export default EnvironmentBanner;
