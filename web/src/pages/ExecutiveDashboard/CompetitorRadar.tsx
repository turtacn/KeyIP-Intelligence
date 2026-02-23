import React from 'react';
import Card from '../../components/ui/Card';
import {
  RadarChart,
  PolarGrid,
  PolarAngleAxis,
  PolarRadiusAxis,
  Radar,
  Legend,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';

interface CompetitorRadarProps {
  data: {
    subject: string;
    A: number;
    B: number;
    fullMark: number;
  }[];
  loading: boolean;
}

const CompetitorRadar: React.FC<CompetitorRadarProps> = ({ data, loading }) => {
  return (
    <Card header="Competitive Radar" padding="none" className="h-80">
      {loading ? (
        <div className="flex items-center justify-center h-full">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        </div>
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <RadarChart outerRadius={90} width={730} height={250} data={data}>
            <PolarGrid />
            <PolarAngleAxis dataKey="subject" />
            <PolarRadiusAxis angle={30} domain={[0, 100]} />
            <Radar
              name="My Organization"
              dataKey="A"
              stroke="#8884d8"
              fill="#8884d8"
              fillOpacity={0.6}
            />
            <Radar
              name="Market Leader"
              dataKey="B"
              stroke="#82ca9d"
              fill="#82ca9d"
              fillOpacity={0.6}
            />
            <Legend />
            <Tooltip />
          </RadarChart>
        </ResponsiveContainer>
      )}
    </Card>
  );
};

export default CompetitorRadar;
