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
	    artist: string;
	    title: string;
	    lyricPath: string;
	    backgroundPath: string;
	    coverPath: string;
	
	    static createFrom(source: any = {}) {
	        return new MusicFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.artist = source["artist"];
	        this.title = source["title"];
	        this.lyricPath = source["lyricPath"];
	        this.backgroundPath = source["backgroundPath"];
	        this.coverPath = source["coverPath"];
	    }
	}

}

