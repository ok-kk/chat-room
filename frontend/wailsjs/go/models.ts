export namespace main {
	
	export class FileRecord {
	    id: string;
	    original_name: string;
	    stored_name: string;
	    file_size: number;
	    file_type: string;
	    uploader_id: string;
	    uploader_name: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new FileRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.original_name = source["original_name"];
	        this.stored_name = source["stored_name"];
	        this.file_size = source["file_size"];
	        this.file_type = source["file_type"];
	        this.uploader_id = source["uploader_id"];
	        this.uploader_name = source["uploader_name"];
	        this.created_at = source["created_at"];
	    }
	}
	export class Message {
	    id: string;
	    room_id: string;
	    sender_id: string;
	    sender_name: string;
	    content: string;
	    msg_type: string;
	    file_url?: string;
	    file_name?: string;
	    file_size?: number;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.room_id = source["room_id"];
	        this.sender_id = source["sender_id"];
	        this.sender_name = source["sender_name"];
	        this.content = source["content"];
	        this.msg_type = source["msg_type"];
	        this.file_url = source["file_url"];
	        this.file_name = source["file_name"];
	        this.file_size = source["file_size"];
	        this.created_at = source["created_at"];
	    }
	}

}

