import React from 'react';

import { Button } from '~/shared/buttons/button';
import { Icon } from '~/shared/icons/icon';
import { Stepper } from '~/shared/stepper/stepper';
import {
  useUnraidActions,
  useUnraidRoute,
  useUnraidIsBusy,
  useUnraidPlan,
} from '~/state/unraid';
import { useLevelSelected, useLevelDisks } from '~/state/level';
import { routeToStep } from '~/helpers/routes';
import { getVariant, getFill } from '~/helpers/styling';
import { useConfigActions, useConfigDryRun } from '~/state/config';
import { Actions } from '~/shared/transfer/actions';

const config = [
  { navTo: 'select', title: 'Select', subtitle: 'Choose folders & disks' },
  { navTo: 'plan', title: 'Plan', subtitle: 'Monitor' },
  { navTo: 'transfer', title: 'Transfer', subtitle: 'Review & Move' },
];

export const Navbar: React.FunctionComponent = () => {
  const route = useUnraidRoute();
  const { transition, levelMove, stop } = useUnraidActions();
  const { toggleDryRun } = useConfigActions();
  const dryRun = useConfigDryRun();
  const busy = useUnraidIsBusy();
  const plan = useUnraidPlan();
  const selected = useLevelSelected();
  const disks = useLevelDisks();

  const onNext = () => transition('next');
  const onPrev = () => transition('prev');
  const onMove = () => levelMove();
  const onDryRun = () => toggleDryRun();
  const onStopPlan = () => stop();

  const currentStep = routeToStep(route);
  const isLast =
    route === '/level/transfer/review' || route === '/level/transfer/operation';
  const nextDisabled =
    busy ||
    isLast ||
    (route === '/level/plan' && !plan) ||
    (route === '/level/select' &&
      (Object.keys(selected).length === 0 || Object.keys(disks).length < 2));

  return (
    <div className="flex flex-row items-center justify-between mb-4">
      <div className="flex justify-start">
        <Button
          label="Prev"
          variant={getVariant(route !== '/level/select')}
          leftIcon={
            <Icon
              name="prev"
              size={20}
              style={getFill(route !== '/level/select')}
            />
          }
          disabled={busy || route === '/level/select'}
          onClick={onPrev}
        />
      </div>

      <div className="flex flex-row flex-1 items-center justify-between">
        <div className="flex flex-row items-center justify-start">
          <span className="mx-2" />
          <Stepper steps={3} currentStep={currentStep} config={config} />
        </div>

        {route === '/level/plan' && busy && (
          <div className="flex flex-row items-center justify-end">
            <Button label="STOP" variant="primary" onClick={onStopPlan} />
            <span className="mx-2" />
          </div>
        )}

        {route === '/level/transfer/review' &&
          plan &&
          plan.bytesToTransfer > 0 && (
            <div className="flex flex-row items-center justify-end">
              <Button label="MOVE" variant="primary" onClick={onMove} />
              <span className="mx-1">|</span>

              <div className="flex items-center">
                <input
                  checked={dryRun}
                  id="checked-checkbox"
                  type="checkbox"
                  className="w-4 h-4 text-blue-600 bg-gray-100 border-gray-300 rounded focus:ring-blue-500 dark:focus:ring-blue-600 dark:ring-offset-gray-800 focus:ring-2 dark:bg-gray-700 dark:border-gray-600"
                  onChange={onDryRun}
                />
                <label
                  htmlFor="checked-checkbox"
                  className="ms-2 text-sm font-medium"
                >
                  dry-run
                </label>
              </div>
              <span className="mx-2" />
            </div>
          )}
        {route === '/level/transfer/operation' && <Actions />}
      </div>

      <div className="flex items-center justify-end">
        <Button
          label="Next"
          variant={getVariant(!isLast)}
          rightIcon={<Icon name="next" size={20} style={getFill(!isLast)} />}
          disabled={nextDisabled}
          onClick={onNext}
        />
      </div>
    </div>
  );
};
