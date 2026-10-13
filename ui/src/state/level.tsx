import { create } from 'zustand';
import { immer } from 'zustand/middleware/immer';

import { Nodes, Node, Targets } from '~/types';
import { Api } from '~/api';
import { decorateNode } from '~/shared/tree/utils';
import { isParent, getAbsolutePath } from '~/helpers/tree';

interface LevelStore {
  source: string;
  tree: Nodes;
  // node id -> path relative to the user share (e.g. tvshows)
  selected: Record<string, string>;
  // disk name -> chosen for levelling
  disks: Targets;
  binDisk: string;
  actions: {
    loadShares: () => Promise<void>;
    loadBranch: (node: Node) => Promise<void>;
    toggleSelected: (node: Node) => void;
    toggleDisk: (name: string) => void;
    toggleAllDisks: (names: string[]) => void;
    setBinDisk: (path: string) => void;
  };
}

const rootNode = {
  id: 'root',
  label: '/',
  leaf: false,
  dir: false,
  parent: '',
};

export const useLevelStore = create<LevelStore>()(
  immer((set, get) => ({
    source: 'user',
    tree: { root: decorateNode(rootNode as Node) },
    selected: {},
    disks: {},
    binDisk: '',

    actions: {
      loadShares: async () => {
        const actions = get().actions;
        actions.loadBranch(decorateNode(rootNode as Node));
      },
      loadBranch: async (node: Node) => {
        set((state) => {
          state.tree[node.id].expanded = !state.tree[node.id].expanded;
        });

        if (isParent(node.id, get().tree)) {
          // change reference to force re-render and show expanded/non-expanded state
          set((state) => {
            state.tree = { ...state.tree };
          });
          return;
        }

        set((state) => {
          state.tree.loader = {
            id: 'loader',
            label: 'loading ...',
            leaf: false,
            dir: false,
            parent: node.id,
            children: [],
            checked: false,
            expanded: false,
            loading: true,
          };
          state.tree[node.id].children = ['loader'];
        });

        const route = `${get().source}/${getAbsolutePath(node, get().tree)}`;
        const branch = await Api.getTree(route, node.id);
        for (const key in branch.nodes) {
          decorateNode(branch.nodes[key]);
        }

        set((state) => {
          delete state.tree.loader;
          state.tree = { ...state.tree, ...branch.nodes };
          state.tree[node.id].children = branch.order;
        });
      },
      toggleSelected: (node: Node) => {
        set((state) => {
          const current = state.tree[node.id];
          if (!current) {
            return;
          }
          const checked = !current.checked;
          current.checked = checked;

          // a folder and one of its parents/children can't both be chosen:
          // whichever was clicked last wins
          let parent = state.tree[current.parent];
          while (parent) {
            if (state.selected[parent.id]) {
              delete state.selected[parent.id];
              parent.checked = false;
            }
            parent = state.tree[parent.parent];
          }

          const removeChildren = (id: string) => {
            (state.tree[id]?.children ?? []).forEach((childId) => {
              const child = state.tree[childId];
              if (!child) {
                return;
              }
              if (state.selected[childId]) {
                delete state.selected[childId];
                child.checked = false;
              }
              removeChildren(childId);
            });
          };
          removeChildren(current.id);

          if (checked) {
            state.selected[current.id] = getAbsolutePath(current, state.tree);
          } else {
            delete state.selected[current.id];
          }
          state.selected = { ...state.selected };
        });
      },
      toggleDisk: (name: string) => {
        set((state) => {
          if (state.disks[name]) {
            delete state.disks[name];
          } else {
            state.disks[name] = true;
          }
        });
      },
      toggleAllDisks: (names: string[]) => {
        set((state) => {
          const all = names.every((name) => state.disks[name]);
          state.disks = {};
          if (!all) {
            names.forEach((name) => (state.disks[name] = true));
          }
        });
      },
      setBinDisk: (path: string) => {
        set((state) => {
          state.binDisk = path;
        });
      },
    },
  })),
);

export const useLevelActions = () => useLevelStore((state) => state.actions);

export const useLevelTree = () => useLevelStore((state) => state.tree);
export const useLevelSelected = () => useLevelStore((state) => state.selected);
export const useLevelDisks = () => useLevelStore((state) => state.disks);
export const useLevelBinDisk = () => useLevelStore((state) => state.binDisk);
