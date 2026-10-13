import React from 'react';

import { Panel } from '~/shared/panel/panel';
import { useUnraidDisks } from '~/state/unraid';
import { useLevelDisks, useLevelActions } from '~/state/level';
import { Checkbox } from '~/shared/checkbox/checkbox';
import { Disk } from '~/shared/disk/disk';
import { Toggle } from '~/flows/scatter/select/toggle';
import { Disk as IDisk } from '~/types';
import { usedPercent, UsedBar } from '../used';

export const Disks: React.FunctionComponent = () => {
  const disks = useUnraidDisks();
  const chosen = useLevelDisks();
  const { toggleDisk, toggleAllDisks } = useLevelActions();

  const names = disks.map((disk) => disk.name);
  const allChecked = names.length > 0 && names.every((name) => chosen[name]);

  const onCheck = (disk: IDisk) => () => toggleDisk(disk.name);
  const onToggleAll = () => toggleAllDisks(names);

  // what the chosen disks would average out to
  const picked = disks.filter((disk) => chosen[disk.name]);
  const size = picked.reduce((sum, disk) => sum + disk.size, 0);
  const free = picked.reduce((sum, disk) => sum + disk.free, 0);
  const subtitle = (
    <div className="flex flex-row items-center gap-4">
      <Toggle allChecked={allChecked} onCheck={onToggleAll} />
      {picked.length > 1 && (
        <span className="text-slate-500 dark:text-gray-500">
          average {usedPercent(size, free).toFixed(0)}% full
        </span>
      )}
    </div>
  );

  return (
    <Panel title="Disks" subtitle={subtitle}>
      {disks.map((disk) => (
        <div key={disk.id} className="flex flex-row items-center py-1">
          <Checkbox checked={!!chosen[disk.name]} onCheck={onCheck(disk)} />
          <div className="pr-4" />
          <div className="flex flex-1 flex-col">
            <Disk disk={disk} />
            <UsedBar size={disk.size} free={disk.free} />
          </div>
        </div>
      ))}
    </Panel>
  );
};
