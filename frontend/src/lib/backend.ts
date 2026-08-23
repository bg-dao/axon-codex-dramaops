import * as ProjectAPI from '../../wailsjs/go/appapi/ProjectAPI';
import * as AgentAPI from '../../wailsjs/go/appapi/AgentAPI';
import * as RuntimeAPI from '../../wailsjs/go/appapi/RuntimeAPI';
import * as SettingsAPI from '../../wailsjs/go/appapi/SettingsAPI';
import * as AssetAPI from '../../wailsjs/go/appapi/AssetAPI';
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime';

export { ProjectAPI, AgentAPI, RuntimeAPI, SettingsAPI, AssetAPI, BrowserOpenURL, EventsOn };

export type ProjectSnapshot = Awaited<ReturnType<typeof ProjectAPI.Current>>;
export type Scene = ProjectSnapshot['scenes'][number];
export type Shot = ProjectSnapshot['shots'][number];
export type Asset = ProjectSnapshot['assets'][number];
export type Run = ProjectSnapshot['runs'][number];
