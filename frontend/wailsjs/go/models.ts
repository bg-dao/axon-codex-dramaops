export namespace appapi {
  export class FountainResult {
    path: string;
    content: string;

    static createFrom(source: any = {}) {
      return new FountainResult(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.path = source["path"];
      this.content = source["content"];
    }
  }
  export class SettingsStatus {
    openaiKeyConfigured: boolean;
    keychainService: string;
    capabilities: provider.Capabilities;

    static createFrom(source: any = {}) {
      return new SettingsStatus(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.openaiKeyConfigured = source["openaiKeyConfigured"];
      this.keychainService = source["keychainService"];
      this.capabilities = this.convertValues(
        source["capabilities"],
        provider.Capabilities,
      );
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class TimelineValidation {
    valid: boolean;
    issues: domain.ContinuityIssue[];
    durationSeconds: number;

    static createFrom(source: any = {}) {
      return new TimelineValidation(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.valid = source["valid"];
      this.issues = this.convertValues(
        source["issues"],
        domain.ContinuityIssue,
      );
      this.durationSeconds = source["durationSeconds"];
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class VoiceBindingStatus {
    profileId: string;
    configured: boolean;
    consentConfigured: boolean;

    static createFrom(source: any = {}) {
      return new VoiceBindingStatus(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.profileId = source["profileId"];
      this.configured = source["configured"];
      this.consentConfigured = source["consentConfigured"];
    }
  }
}

export namespace approval {
  export class Decision {
    requestId: string;
    approved: boolean;
    // Go type: time
    decidedAt: any;

    static createFrom(source: any = {}) {
      return new Decision(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.requestId = source["requestId"];
      this.approved = source["approved"];
      this.decidedAt = this.convertValues(source["decidedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class Request {
    id: string;
    action: string;
    summary: string;
    details?: Record<string, any>;
    // Go type: time
    requestedAt: any;

    static createFrom(source: any = {}) {
      return new Request(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.action = source["action"];
      this.summary = source["summary"];
      this.details = source["details"];
      this.requestedAt = this.convertValues(source["requestedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
}

export namespace appserver {
  export class Turn {
    id: string;
    status: string;

    static createFrom(source: any = {}) {
      return new Turn(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.status = source["status"];
    }
  }
}

export namespace domain {
  export class Provenance {
    provider: string;
    model?: string;
    prompt?: string;
    parameters?: Record<string, any>;
    providerRequestId?: string;
    toolVersion?: string;
    // Go type: time
    generatedAt?: any;

    static createFrom(source: any = {}) {
      return new Provenance(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.provider = source["provider"];
      this.model = source["model"];
      this.prompt = source["prompt"];
      this.parameters = source["parameters"];
      this.providerRequestId = source["providerRequestId"];
      this.toolVersion = source["toolVersion"];
      this.generatedAt = this.convertValues(source["generatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class MediaInfo {
    durationSeconds?: number;
    width?: number;
    height?: number;
    fps?: number;
    videoCodec?: string;
    audioCodec?: string;
    sampleRate?: number;
    channels?: number;

    static createFrom(source: any = {}) {
      return new MediaInfo(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.durationSeconds = source["durationSeconds"];
      this.width = source["width"];
      this.height = source["height"];
      this.fps = source["fps"];
      this.videoCodec = source["videoCodec"];
      this.audioCodec = source["audioCodec"];
      this.sampleRate = source["sampleRate"];
      this.channels = source["channels"];
    }
  }
  export class AssetInput {
    assetId: string;
    role: string;

    static createFrom(source: any = {}) {
      return new AssetInput(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.assetId = source["assetId"];
      this.role = source["role"];
    }
  }
  export class Asset {
    schemaVersion: number;
    id: string;
    episodeId?: string;
    shotId?: string;
    scriptBlockId?: string;
    kind: string;
    relativePath: string;
    sha256: string;
    inputs?: AssetInput[];
    runId?: string;
    mediaInfo?: MediaInfo;
    provenance: Provenance;
    // Go type: time
    createdAt: any;

    static createFrom(source: any = {}) {
      return new Asset(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.episodeId = source["episodeId"];
      this.shotId = source["shotId"];
      this.scriptBlockId = source["scriptBlockId"];
      this.kind = source["kind"];
      this.relativePath = source["relativePath"];
      this.sha256 = source["sha256"];
      this.inputs = this.convertValues(source["inputs"], AssetInput);
      this.runId = source["runId"];
      this.mediaInfo = this.convertValues(source["mediaInfo"], MediaInfo);
      this.provenance = this.convertValues(source["provenance"], Provenance);
      this.createdAt = this.convertValues(source["createdAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }

  export class AudioCue {
    id: string;
    lane: string;
    assetId: string;
    scriptBlockId?: string;
    startSeconds: number;
    durationSeconds: number;
    gainDb?: number;
    loop?: boolean;
    duckBgm?: boolean;

    static createFrom(source: any = {}) {
      return new AudioCue(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.lane = source["lane"];
      this.assetId = source["assetId"];
      this.scriptBlockId = source["scriptBlockId"];
      this.startSeconds = source["startSeconds"];
      this.durationSeconds = source["durationSeconds"];
      this.gainDb = source["gainDb"];
      this.loop = source["loop"];
      this.duckBgm = source["duckBgm"];
    }
  }
  export class VoiceProfile {
    id: string;
    kind: string;
    name: string;
    builtInVoice?: string;
    externalAssetId?: string;
    consentConfirmed?: boolean;

    static createFrom(source: any = {}) {
      return new VoiceProfile(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.kind = source["kind"];
      this.name = source["name"];
      this.builtInVoice = source["builtInVoice"];
      this.externalAssetId = source["externalAssetId"];
      this.consentConfirmed = source["consentConfirmed"];
    }
  }
  export class Character {
    schemaVersion: number;
    id: string;
    name: string;
    description?: string;
    appearance?: string;
    wardrobe?: string;
    negativePrompt?: string;
    referenceAssets?: string[];
    voiceProfile: VoiceProfile;
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Character(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.name = source["name"];
      this.description = source["description"];
      this.appearance = source["appearance"];
      this.wardrobe = source["wardrobe"];
      this.negativePrompt = source["negativePrompt"];
      this.referenceAssets = source["referenceAssets"];
      this.voiceProfile = this.convertValues(
        source["voiceProfile"],
        VoiceProfile,
      );
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class ContinuityIssue {
    code: string;
    severity: string;
    episodeId?: string;
    sceneId?: string;
    shotId?: string;
    message: string;

    static createFrom(source: any = {}) {
      return new ContinuityIssue(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.code = source["code"];
      this.severity = source["severity"];
      this.episodeId = source["episodeId"];
      this.sceneId = source["sceneId"];
      this.shotId = source["shotId"];
      this.message = source["message"];
    }
  }
  export class ScriptBlock {
    id: string;
    sceneId: string;
    kind: string;
    order: number;
    characterId?: string;
    text: string;
    emotion?: string;
    selectedVoiceAssetId?: string;

    static createFrom(source: any = {}) {
      return new ScriptBlock(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.sceneId = source["sceneId"];
      this.kind = source["kind"];
      this.order = source["order"];
      this.characterId = source["characterId"];
      this.text = source["text"];
      this.emotion = source["emotion"];
      this.selectedVoiceAssetId = source["selectedVoiceAssetId"];
    }
  }
  export class Episode {
    schemaVersion: number;
    id: string;
    number: number;
    title: string;
    logline?: string;
    synopsis?: string;
    status: string;
    sceneIds: string[];
    scriptBlocks: ScriptBlock[];
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Episode(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.number = source["number"];
      this.title = source["title"];
      this.logline = source["logline"];
      this.synopsis = source["synopsis"];
      this.status = source["status"];
      this.sceneIds = source["sceneIds"];
      this.scriptBlocks = this.convertValues(
        source["scriptBlocks"],
        ScriptBlock,
      );
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class OutputSettings {
    orientation: string;
    width: number;
    height: number;
    fps: number;
    videoCodec: string;
    audioCodec: string;
    audioSampleRate: number;
    audioChannels: number;
    loudnessLufs: number;
    truePeakDbtp: number;
    burnSubtitles: boolean;
    subtitleSafeArea: number;

    static createFrom(source: any = {}) {
      return new OutputSettings(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.orientation = source["orientation"];
      this.width = source["width"];
      this.height = source["height"];
      this.fps = source["fps"];
      this.videoCodec = source["videoCodec"];
      this.audioCodec = source["audioCodec"];
      this.audioSampleRate = source["audioSampleRate"];
      this.audioChannels = source["audioChannels"];
      this.loudnessLufs = source["loudnessLufs"];
      this.truePeakDbtp = source["truePeakDbtp"];
      this.burnSubtitles = source["burnSubtitles"];
      this.subtitleSafeArea = source["subtitleSafeArea"];
    }
  }
  export class SubtitleCue {
    id: string;
    scriptBlockId?: string;
    startSeconds: number;
    durationSeconds: number;
    text: string;

    static createFrom(source: any = {}) {
      return new SubtitleCue(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.scriptBlockId = source["scriptBlockId"];
      this.startSeconds = source["startSeconds"];
      this.durationSeconds = source["durationSeconds"];
      this.text = source["text"];
    }
  }
  export class VideoClip {
    id: string;
    shotId: string;
    assetId: string;
    order: number;
    inSeconds: number;
    outSeconds: number;
    transition: string;
    transitionSeconds?: number;
    fit: string;

    static createFrom(source: any = {}) {
      return new VideoClip(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.shotId = source["shotId"];
      this.assetId = source["assetId"];
      this.order = source["order"];
      this.inSeconds = source["inSeconds"];
      this.outSeconds = source["outSeconds"];
      this.transition = source["transition"];
      this.transitionSeconds = source["transitionSeconds"];
      this.fit = source["fit"];
    }
  }
  export class EpisodeEdit {
    schemaVersion: number;
    episodeId: string;
    videoTrack: VideoClip[];
    audioCues: AudioCue[];
    subtitleCues: SubtitleCue[];
    output: OutputSettings;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new EpisodeEdit(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.episodeId = source["episodeId"];
      this.videoTrack = this.convertValues(source["videoTrack"], VideoClip);
      this.audioCues = this.convertValues(source["audioCues"], AudioCue);
      this.subtitleCues = this.convertValues(
        source["subtitleCues"],
        SubtitleCue,
      );
      this.output = this.convertValues(source["output"], OutputSettings);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class Location {
    schemaVersion: number;
    id: string;
    name: string;
    description?: string;
    continuityNotes?: string;
    referenceAssets?: string[];
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Location(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.name = source["name"];
      this.description = source["description"];
      this.continuityNotes = source["continuityNotes"];
      this.referenceAssets = source["referenceAssets"];
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }

  export class SoundPalette {
    ambienceAssetIds?: string[];
    bgmAssetIds?: string[];
    motifs?: Record<string, string>;
    notes?: string;

    static createFrom(source: any = {}) {
      return new SoundPalette(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.ambienceAssetIds = source["ambienceAssetIds"];
      this.bgmAssetIds = source["bgmAssetIds"];
      this.motifs = source["motifs"];
      this.notes = source["notes"];
    }
  }
  export class StyleBible {
    visualStyle?: string;
    colorPalette?: string[];
    lightingRules?: string;
    negativePrompt?: string;
    referenceAssets?: string[];

    static createFrom(source: any = {}) {
      return new StyleBible(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.visualStyle = source["visualStyle"];
      this.colorPalette = source["colorPalette"];
      this.lightingRules = source["lightingRules"];
      this.negativePrompt = source["negativePrompt"];
      this.referenceAssets = source["referenceAssets"];
    }
  }
  export class Project {
    schemaVersion: number;
    id: string;
    name: string;
    contentLanguage: string;
    activeEpisodeId?: string;
    activeThreadId?: string;
    styleBible: StyleBible;
    soundPalette: SoundPalette;
    output: OutputSettings;
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Project(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.name = source["name"];
      this.contentLanguage = source["contentLanguage"];
      this.activeEpisodeId = source["activeEpisodeId"];
      this.activeThreadId = source["activeThreadId"];
      this.styleBible = this.convertValues(source["styleBible"], StyleBible);
      this.soundPalette = this.convertValues(
        source["soundPalette"],
        SoundPalette,
      );
      this.output = this.convertValues(source["output"], OutputSettings);
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class Prop {
    schemaVersion: number;
    id: string;
    name: string;
    description?: string;
    continuityState?: string;
    referenceAssets?: string[];
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Prop(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.name = source["name"];
      this.description = source["description"];
      this.continuityState = source["continuityState"];
      this.referenceAssets = source["referenceAssets"];
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }

  export class Run {
    schemaVersion: number;
    id: string;
    operation: string;
    status: string;
    episodeId?: string;
    shotId?: string;
    scriptBlockId?: string;
    providerJobId?: string;
    progress?: number;
    error?: string;
    metadata?: Record<string, any>;
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Run(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.operation = source["operation"];
      this.status = source["status"];
      this.episodeId = source["episodeId"];
      this.shotId = source["shotId"];
      this.scriptBlockId = source["scriptBlockId"];
      this.providerJobId = source["providerJobId"];
      this.progress = source["progress"];
      this.error = source["error"];
      this.metadata = source["metadata"];
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class Scene {
    schemaVersion: number;
    id: string;
    episodeId: string;
    title: string;
    summary?: string;
    locationId?: string;
    timeOfDay?: string;
    order: number;
    shotIds: string[];
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Scene(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.episodeId = source["episodeId"];
      this.title = source["title"];
      this.summary = source["summary"];
      this.locationId = source["locationId"];
      this.timeOfDay = source["timeOfDay"];
      this.order = source["order"];
      this.shotIds = source["shotIds"];
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }

  export class Shot {
    schemaVersion: number;
    id: string;
    episodeId: string;
    sceneId: string;
    title: string;
    order: number;
    scriptBlockIds?: string[];
    prompt?: string;
    durationSeconds: number;
    aspectRatio: string;
    shotSize: string;
    cameraAngle: string;
    cameraMovement: string;
    lensMm?: number;
    composition?: string;
    focusSubject?: string;
    blocking?: string;
    lighting?: string;
    screenDirection?: string;
    eyeLine?: string;
    characterIds?: string[];
    propIds?: string[];
    wardrobeContinuity?: string;
    propContinuity?: string;
    transition: string;
    referenceAssets?: string[];
    selectedKeyframeAssetId?: string;
    selectedVideoAssetId?: string;
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Shot(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.schemaVersion = source["schemaVersion"];
      this.id = source["id"];
      this.episodeId = source["episodeId"];
      this.sceneId = source["sceneId"];
      this.title = source["title"];
      this.order = source["order"];
      this.scriptBlockIds = source["scriptBlockIds"];
      this.prompt = source["prompt"];
      this.durationSeconds = source["durationSeconds"];
      this.aspectRatio = source["aspectRatio"];
      this.shotSize = source["shotSize"];
      this.cameraAngle = source["cameraAngle"];
      this.cameraMovement = source["cameraMovement"];
      this.lensMm = source["lensMm"];
      this.composition = source["composition"];
      this.focusSubject = source["focusSubject"];
      this.blocking = source["blocking"];
      this.lighting = source["lighting"];
      this.screenDirection = source["screenDirection"];
      this.eyeLine = source["eyeLine"];
      this.characterIds = source["characterIds"];
      this.propIds = source["propIds"];
      this.wardrobeContinuity = source["wardrobeContinuity"];
      this.propContinuity = source["propContinuity"];
      this.transition = source["transition"];
      this.referenceAssets = source["referenceAssets"];
      this.selectedKeyframeAssetId = source["selectedKeyframeAssetId"];
      this.selectedVideoAssetId = source["selectedVideoAssetId"];
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class Snapshot {
    root: string;
    project: Project;
    episodes: Episode[];
    characters: Character[];
    locations: Location[];
    props: Prop[];
    scenes: Scene[];
    shots: Shot[];
    edits: EpisodeEdit[];
    assets: Asset[];
    runs: Run[];
    continuityIssues: ContinuityIssue[];

    static createFrom(source: any = {}) {
      return new Snapshot(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.root = source["root"];
      this.project = this.convertValues(source["project"], Project);
      this.episodes = this.convertValues(source["episodes"], Episode);
      this.characters = this.convertValues(source["characters"], Character);
      this.locations = this.convertValues(source["locations"], Location);
      this.props = this.convertValues(source["props"], Prop);
      this.scenes = this.convertValues(source["scenes"], Scene);
      this.shots = this.convertValues(source["shots"], Shot);
      this.edits = this.convertValues(source["edits"], EpisodeEdit);
      this.assets = this.convertValues(source["assets"], Asset);
      this.runs = this.convertValues(source["runs"], Run);
      this.continuityIssues = this.convertValues(
        source["continuityIssues"],
        ContinuityIssue,
      );
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
}

export namespace exporter {
  export class Result {
    path: string;
    sha256: string;
    files: number;

    static createFrom(source: any = {}) {
      return new Result(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.path = source["path"];
      this.sha256 = source["sha256"];
      this.files = source["files"];
    }
  }
}

export namespace media {
  export class Result {
    run: domain.Run;
    job: provider.Job;
    asset?: domain.Asset;

    static createFrom(source: any = {}) {
      return new Result(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.run = this.convertValues(source["run"], domain.Run);
      this.job = this.convertValues(source["job"], provider.Job);
      this.asset = this.convertValues(source["asset"], domain.Asset);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
}

export namespace project {
  export class CreateOptions {
    name: string;
    contentLanguage: string;
    orientation: string;

    static createFrom(source: any = {}) {
      return new CreateOptions(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.name = source["name"];
      this.contentLanguage = source["contentLanguage"];
      this.orientation = source["orientation"];
    }
  }
}

export namespace provider {
  export class Capabilities {
    imageGeneration: boolean;
    imageReferences: boolean;
    maxImageReferences?: number;
    videoGeneration: boolean;
    videoExperimental: boolean;
    videoReferenceRoles?: string[];
    maxVideoReferences?: number;
    speechGeneration: boolean;
    customVoices: boolean;
    soundGeneration: boolean;
    imageModels?: string[];
    videoModels?: string[];
    speechModels?: string[];
    builtInVoices?: string[];
    reason?: string;
    videoNotice?: string;

    static createFrom(source: any = {}) {
      return new Capabilities(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.imageGeneration = source["imageGeneration"];
      this.imageReferences = source["imageReferences"];
      this.maxImageReferences = source["maxImageReferences"];
      this.videoGeneration = source["videoGeneration"];
      this.videoExperimental = source["videoExperimental"];
      this.videoReferenceRoles = source["videoReferenceRoles"];
      this.maxVideoReferences = source["maxVideoReferences"];
      this.speechGeneration = source["speechGeneration"];
      this.customVoices = source["customVoices"];
      this.soundGeneration = source["soundGeneration"];
      this.imageModels = source["imageModels"];
      this.videoModels = source["videoModels"];
      this.speechModels = source["speechModels"];
      this.builtInVoices = source["builtInVoices"];
      this.reason = source["reason"];
      this.videoNotice = source["videoNotice"];
    }
  }
  export class Reference {
    assetId: string;
    role: string;

    static createFrom(source: any = {}) {
      return new Reference(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.assetId = source["assetId"];
      this.role = source["role"];
    }
  }
  export class ImageRequest {
    prompt: string;
    model?: string;
    size?: string;
    quality?: string;
    references?: Reference[];
    parameters?: Record<string, any>;

    static createFrom(source: any = {}) {
      return new ImageRequest(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.prompt = source["prompt"];
      this.model = source["model"];
      this.size = source["size"];
      this.quality = source["quality"];
      this.references = this.convertValues(source["references"], Reference);
      this.parameters = source["parameters"];
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class Job {
    id: string;
    kind: string;
    status: string;
    progress?: number;
    providerRequestId?: string;
    error?: string;
    // Go type: time
    createdAt: any;
    // Go type: time
    updatedAt: any;

    static createFrom(source: any = {}) {
      return new Job(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.kind = source["kind"];
      this.status = source["status"];
      this.progress = source["progress"];
      this.providerRequestId = source["providerRequestId"];
      this.error = source["error"];
      this.createdAt = this.convertValues(source["createdAt"], null);
      this.updatedAt = this.convertValues(source["updatedAt"], null);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }

  export class SpeechRequest {
    text: string;
    model?: string;
    voice?: string;
    voiceProfileId: string;
    instructions?: string;
    responseFormat?: string;
    speed?: number;
    parameters?: Record<string, any>;

    static createFrom(source: any = {}) {
      return new SpeechRequest(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.text = source["text"];
      this.model = source["model"];
      this.voice = source["voice"];
      this.voiceProfileId = source["voiceProfileId"];
      this.instructions = source["instructions"];
      this.responseFormat = source["responseFormat"];
      this.speed = source["speed"];
      this.parameters = source["parameters"];
    }
  }
  export class VideoRequest {
    prompt: string;
    model?: string;
    seconds?: number;
    size?: string;
    references?: Reference[];
    parameters?: Record<string, any>;

    static createFrom(source: any = {}) {
      return new VideoRequest(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.prompt = source["prompt"];
      this.model = source["model"];
      this.seconds = source["seconds"];
      this.size = source["size"];
      this.references = this.convertValues(source["references"], Reference);
      this.parameters = source["parameters"];
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
}

export namespace render {
  export class RuntimeStatus {
    ffmpegPath?: string;
    ffprobePath?: string;
    version?: string;
    compatible: boolean;
    encoder?: string;
    source?: string;
    error?: string;

    static createFrom(source: any = {}) {
      return new RuntimeStatus(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.ffmpegPath = source["ffmpegPath"];
      this.ffprobePath = source["ffprobePath"];
      this.version = source["version"];
      this.compatible = source["compatible"];
      this.encoder = source["encoder"];
      this.source = source["source"];
      this.error = source["error"];
    }
  }
}

export namespace runtime {
  export class Status {
    path?: string;
    version?: string;
    required: string;
    source?: string;
    compatible: boolean;
    error?: string;

    static createFrom(source: any = {}) {
      return new Status(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.path = source["path"];
      this.version = source["version"];
      this.required = source["required"];
      this.source = source["source"];
      this.compatible = source["compatible"];
      this.error = source["error"];
    }
  }
}
