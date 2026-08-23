export namespace appapi {

	export class SettingsStatus {
	    openaiKeyConfigured: boolean;
	    keychainService: string;
	    capabilities: provider.Capabilities;

	    static createFrom(source: any = {}) {
	        return new SettingsStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.openaiKeyConfigured = source["openaiKeyConfigured"];
	        this.keychainService = source["keychainService"];
	        this.capabilities = this.convertValues(source["capabilities"], provider.Capabilities);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestId = source["requestId"];
	        this.approved = source["approved"];
	        this.decidedAt = this.convertValues(source["decidedAt"], null);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        if ('string' === typeof source) source = JSON.parse(source);
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
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        if ('string' === typeof source) source = JSON.parse(source);
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
	    // Go type: time
	    generatedAt?: any;

	    static createFrom(source: any = {}) {
	        return new Provenance(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.prompt = source["prompt"];
	        this.parameters = source["parameters"];
	        this.providerRequestId = source["providerRequestId"];
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	export class Asset {
	    schemaVersion: number;
	    id: string;
	    shotId?: string;
	    kind: string;
	    relativePath: string;
	    sha256: string;
	    parentAssetId?: string;
	    runId?: string;
	    provenance: Provenance;
	    // Go type: time
	    createdAt: any;

	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.shotId = source["shotId"];
	        this.kind = source["kind"];
	        this.relativePath = source["relativePath"];
	        this.sha256 = source["sha256"];
	        this.parentAssetId = source["parentAssetId"];
	        this.runId = source["runId"];
	        this.provenance = this.convertValues(source["provenance"], Provenance);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	export class Project {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    activeThreadId?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;

	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.activeThreadId = source["activeThreadId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    shotId?: string;
	    providerJobId?: string;
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
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.operation = source["operation"];
	        this.status = source["status"];
	        this.shotId = source["shotId"];
	        this.providerJobId = source["providerJobId"];
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
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    title: string;
	    summary?: string;
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
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.summary = source["summary"];
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
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    sceneId: string;
	    title: string;
	    order: number;
	    prompt?: string;
	    durationSeconds?: number;
	    aspectRatio?: string;
	    parameters?: Record<string, any>;
	    referenceAssets?: string[];
	    selectedAssetId?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;

	    static createFrom(source: any = {}) {
	        return new Shot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.sceneId = source["sceneId"];
	        this.title = source["title"];
	        this.order = source["order"];
	        this.prompt = source["prompt"];
	        this.durationSeconds = source["durationSeconds"];
	        this.aspectRatio = source["aspectRatio"];
	        this.parameters = source["parameters"];
	        this.referenceAssets = source["referenceAssets"];
	        this.selectedAssetId = source["selectedAssetId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	    brief: string;
	    project: Project;
	    scenes: Scene[];
	    shots: Shot[];
	    assets: Asset[];
	    runs: Run[];

	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.brief = source["brief"];
	        this.project = this.convertValues(source["project"], Project);
	        this.scenes = this.convertValues(source["scenes"], Scene);
	        this.shots = this.convertValues(source["shots"], Shot);
	        this.assets = this.convertValues(source["assets"], Asset);
	        this.runs = this.convertValues(source["runs"], Run);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	        if ('string' === typeof source) source = JSON.parse(source);
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
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.run = this.convertValues(source["run"], domain.Run);
	        this.job = this.convertValues(source["job"], provider.Job);
	        this.asset = this.convertValues(source["asset"], domain.Asset);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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

export namespace provider {

	export class Capabilities {
	    imageGeneration: boolean;
	    videoGeneration: boolean;
	    videoReferences: boolean;
	    videoExperimental: boolean;
	    imageModels: string[];
	    videoModels: string[];
	    reason?: string;
	    videoNotice?: string;

	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imageGeneration = source["imageGeneration"];
	        this.videoGeneration = source["videoGeneration"];
	        this.videoReferences = source["videoReferences"];
	        this.videoExperimental = source["videoExperimental"];
	        this.imageModels = source["imageModels"];
	        this.videoModels = source["videoModels"];
	        this.reason = source["reason"];
	        this.videoNotice = source["videoNotice"];
	    }
	}
	export class ImageRequest {
	    prompt: string;
	    model?: string;
	    size?: string;
	    quality?: string;
	    referencePaths?: string[];
	    parameters?: Record<string, any>;

	    static createFrom(source: any = {}) {
	        return new ImageRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt = source["prompt"];
	        this.model = source["model"];
	        this.size = source["size"];
	        this.quality = source["quality"];
	        this.referencePaths = source["referencePaths"];
	        this.parameters = source["parameters"];
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
	        if ('string' === typeof source) source = JSON.parse(source);
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
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
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
	export class VideoRequest {
	    prompt: string;
	    model?: string;
	    seconds?: number;
	    size?: string;
	    referenceAssetId?: string;
	    parameters?: Record<string, any>;

	    static createFrom(source: any = {}) {
	        return new VideoRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt = source["prompt"];
	        this.model = source["model"];
	        this.seconds = source["seconds"];
	        this.size = source["size"];
	        this.referenceAssetId = source["referenceAssetId"];
	        this.parameters = source["parameters"];
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
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.version = source["version"];
	        this.required = source["required"];
	        this.source = source["source"];
	        this.compatible = source["compatible"];
	        this.error = source["error"];
	    }
	}

}
