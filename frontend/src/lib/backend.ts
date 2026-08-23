import * as ProjectAPI from "../../wailsjs/go/appapi/ProjectAPI";
import * as AgentAPI from "../../wailsjs/go/appapi/AgentAPI";
import * as RuntimeAPI from "../../wailsjs/go/appapi/RuntimeAPI";
import * as SettingsAPI from "../../wailsjs/go/appapi/SettingsAPI";
import * as AssetAPI from "../../wailsjs/go/appapi/AssetAPI";
import * as StoryAPI from "../../wailsjs/go/appapi/StoryAPI";
import * as RenderAPI from "../../wailsjs/go/appapi/RenderAPI";
import { domain, project, provider } from "../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";

export {
  ProjectAPI,
  StoryAPI,
  AssetAPI,
  RenderAPI,
  AgentAPI,
  RuntimeAPI,
  SettingsAPI,
  BrowserOpenURL,
  EventsOn,
  domain,
  project,
  provider,
};

export type ProjectSnapshot = Awaited<ReturnType<typeof ProjectAPI.Current>>;
export type Scene = ProjectSnapshot["scenes"][number];
export type Shot = ProjectSnapshot["shots"][number];
export type Asset = ProjectSnapshot["assets"][number];
export type Run = ProjectSnapshot["runs"][number];
export type Episode = ProjectSnapshot["episodes"][number];
export type Character = ProjectSnapshot["characters"][number];
export type Location = ProjectSnapshot["locations"][number];
export type Prop = ProjectSnapshot["props"][number];
export type EpisodeEdit = ProjectSnapshot["edits"][number];
