import React from 'react';

import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from '@/components/ui/resizable';

import { Panel } from '~/shared/panel/panel';
import { Selectable } from '~/shared/selectable/selectable';
import { Disk } from '~/shared/disk/disk';
import { useUnraidPlan, useUnraidDisks } from '~/state/unraid';
import { useLevelDisks, useLevelBinDisk, useLevelActions } from '~/state/level';
import { humanBytes } from '~/helpers/units';
import { Disk as IDisk } from '~/types';
import { UsedBar } from '../used';

const Disks: React.FunctionComponent = () => {
  const plan = useUnraidPlan();
  const disks = useUnraidDisks();
  const chosen = useLevelDisks();
  const binDisk = useLevelBinDisk();
  const { setBinDisk } = useLevelActions();

  if (!plan) {
    return (
      <Panel title="Disks">
        <span className="text-sm">No plan available.</span>
      </Panel>
    );
  }

  const onSelect = (disk: IDisk) => () => setBinDisk(disk.path);

  const subtitle = (
    <span className="text-slate-500 dark:text-gray-500">
      {humanBytes(plan.bytesToTransfer)} to move
    </span>
  );

  return (
    <Panel title="Disks" subtitle={subtitle}>
      {disks
        .filter((disk) => {
          // a plan recovered after a reload has no selection to go by, so
          // fall back to the disks the plan touches
          const vdisk = plan.vdisks[disk.path];
          return vdisk && (chosen[disk.name] || vdisk.src || vdisk.dst);
        })
        .map((disk) => {
          const vdisk = plan.vdisks[disk.path];
          const role = vdisk.src ? 'gives' : vdisk.dst ? 'receives' : 'stays';
          return (
            <Selectable
              key={disk.id}
              onClick={onSelect(disk)}
              selected={disk.path === binDisk}
            >
              <div className="flex flex-col">
                <div className="flex flex-row items-center gap-2">
                  <Disk disk={disk} />
                  <span className="text-xs w-16 text-right text-slate-500 dark:text-gray-500">
                    {role}
                  </span>
                </div>
                <UsedBar
                  size={disk.size}
                  free={vdisk.currentFree}
                  label="now"
                />
                <UsedBar
                  size={disk.size}
                  free={vdisk.plannedFree}
                  label="planned"
                />
              </div>
            </Selectable>
          );
        })}
    </Panel>
  );
};

const Incoming: React.FunctionComponent = () => {
  const plan = useUnraidPlan();
  const disks = useUnraidDisks();
  const binDisk = useLevelBinDisk();

  const bin = plan && binDisk !== '' ? plan.vdisks[binDisk]?.bin : null;
  const nameOf = (path: string) =>
    disks.find((disk) => disk.path === path)?.name ?? path;

  return (
    <Panel title="Incoming items">
      {!bin || !bin.items ? (
        <span className="text-sm">
          {binDisk === ''
            ? 'Click on a disk to see what it will receive.'
            : 'This disk receives nothing.'}
        </span>
      ) : (
        bin.items.map((item) => (
          <div
            key={`${item.location}/${item.path}`}
            className="whitespace-nowrap"
          >
            <span className="text-gray-700 dark:text-gray-500 text-sm">
              ({humanBytes(item.size)}){' '}
            </span>
            <span className="text-gray-500 dark:text-gray-700 text-sm">
              {item.path}
            </span>
            <span className="text-gray-700 dark:text-gray-500 text-sm">
              {' '}
              from {nameOf(item.location)}
            </span>
          </div>
        ))
      )}
    </Panel>
  );
};

export const Review: React.FunctionComponent = () => {
  return (
    <div className="flex flex-1">
      <ResizablePanelGroup direction="horizontal" className="flex flex-1">
        <ResizablePanel defaultSize={55}>
          <Disks />
        </ResizablePanel>
        <ResizableHandle withHandle />
        <ResizablePanel defaultSize={45}>
          <Incoming />
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  );
};
