export namespace main {
	
	export class LyricResult {
	    content: string;
	    lyricPath: string;
	
	    static createFrom(source: any = {}) {
	        return new LyricResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.lyricPath = source["lyricPath"];
	    }
	}
	export class MusicFile {
	    name: string;
	    path: string;
	    size: string;
	    quality: string;
	    artist: string;
	    title: string;
	    lyricPath: string;
	    backgroundPath: string;
	    coverPath: string;
	    thumbnailPath: string;
	    isVideo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MusicFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.quality = source["quality"];
	        this.artist = source["artist"];
	        this.title = source["title"];
	        this.lyricPath = source["lyricPath"];
	        this.backgroundPath = source["backgroundPath"];
	        this.coverPath = source["coverPath"];
	        this.thumbnailPath = source["thumbnailPath"];
	        this.isVideo = source["isVideo"];
	    }
	}
	export class MediaFilesResult {
	    files: MusicFile[];
	    isVideoMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MediaFilesResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], MusicFile);
	        this.isVideoMode = source["isVideoMode"];
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

