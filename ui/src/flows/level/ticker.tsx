import React from 'react';

import { useUnraidRoute } from '~/state/unraid';
import { Line } from '~/shared/transfer/line';

export const Ticker: React.FunctionComponent = () => {
  const route = useUnraidRoute();

  return (
    <div>
      {(route === '/level/select' || route === '/level') && (
        <span>
          Choose the <span className="font-bold">folders</span> to level and{' '}
          <span className="font-bold">TWO or MORE</span> disks, then click next.
          Each folder inside a chosen one (e.g. every show inside tvshows) is
          kept whole on a single disk.
        </span>
      )}
      {route === '/level/plan' && (
        <span>Monitor the progress of the level planning operation.</span>
      )}
      {route === '/level/transfer/review' && (
        <span>
          Review the planned fill of each disk and what it will receive, then
          click on <span className="font-bold">MOVE</span>.
          <br />
          <span className="font-bold">*NOTE</span>: Data only moves from disks
          above the average fill to disks below it, so no disk both gives and
          receives.
        </span>
      )}
      {route === '/level/transfer/operation' && <Line />}
    </div>
  );
};
