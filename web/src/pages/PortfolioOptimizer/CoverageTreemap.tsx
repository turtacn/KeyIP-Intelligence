import React from 'react';
import Card from '../../components/ui/Card';
import { useTranslation } from 'react-i18next';

interface CoverageTreemapProps {
  data: { [key: string]: number }; // category -> count
  scores: { [key: string]: number }; // category -> avg score (0-100)
}

const CoverageTreemap: React.FC<CoverageTreemapProps> = ({ data, scores }) => {
  const { t } = useTranslation();
  const total = Object.values(data).reduce((acc, curr) => acc + curr, 0);

  // Sort by size desc for simple layout (not true treemap algorithm but close enough for grid)
  const sortedCategories = Object.entries(data).sort((a, b) => b[1] - a[1]);

  return (
    <Card header={t('portfolio.treemap.title', 'Technology Coverage Map')} padding="none" className="h-[400px] flex flex-col overflow-hidden">
      <div className="flex-1 flex flex-wrap content-start p-1">
        {sortedCategories.map(([category, count]) => {
          const percentage = (count / total) * 100;
          // Simple grid approximation: width proportional to percentage but constrained
          // For a true treemap without library, we can use flex-grow or grid-template-areas.
          // Here, let's use flex-grow with min-width/height constraints.

          const score = scores[category] || 50;
          // Color scale: Blue (low) -> Purple (high) or just intensity of blue
          // Intensity: 0 -> bg-blue-100, 100 -> bg-blue-900
          // We'll use opacity on a base color for simplicity and Tailwind classes

          // Removed unused bgColorClass
          let opacity = 0.2 + (score / 100) * 0.8;

          return (
            <div
              key={category}
              className="p-1 transition-all duration-300 hover:scale-[1.02] cursor-pointer"
              style={{
                flexGrow: count,
                flexBasis: `${percentage}%`, // Approximate width
                minWidth: '100px',
                minHeight: '80px'
              }}
              title={`${category}: ${count} patents, Avg Score: ${score}`}
            >
              <div
                className="w-full h-full rounded-md flex flex-col items-center justify-center text-white text-center shadow-sm border border-white/20 relative overflow-hidden group"
                style={{ backgroundColor: `rgba(59, 130, 246, ${opacity})` }} // blue-500
              >
                <span className="font-bold text-sm drop-shadow-md z-10">{category}</span>
                <span className="text-xs opacity-80 drop-shadow-md z-10">{count} patents</span>
                <span className="text-xs opacity-0 group-hover:opacity-100 transition-opacity absolute bottom-2 z-10">Score: {score}</span>
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
};

export default CoverageTreemap;
