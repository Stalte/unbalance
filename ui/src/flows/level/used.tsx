import React from 'react';

export const usedPercent = (size: number, free: number) =>
  size > 0 ? ((size - free) / size) * 100 : 0;

interface UsedBarProps {
  size: number;
  free: number;
  label?: string;
}

export const UsedBar: React.FunctionComponent<UsedBarProps> = ({
  size,
  free,
  label = 'used',
}) => {
  const used = usedPercent(size, free);

  return (
    <div className="flex flex-row items-center font-mono text-xs text-sky-700 dark:text-slate-500">
      <span className="pr-2 w-28 whitespace-nowrap">
        {label} {used.toFixed(0)}%
      </span>
      <div className="flex flex-1">
        <div className="w-full rounded bg-gray-400 dark:bg-gray-800">
          <div
            className="p-0.5 leading-none rounded bg-sky-700 dark:bg-slate-500"
            style={{ width: `${used}%` }}
          ></div>
        </div>
      </div>
    </div>
  );
};
