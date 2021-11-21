function errorMessage(el, msg) {
    var originalMsg = el.innerHTML
    el.innerHTML = msg
    el.classList.add("error")
    setTimeout(e=>{
        el.classList.remove("error")
        el.innerHTML = originalMsg
    }, 20000)
}

var upload = {
    dropbox: null, // HTML upload zone
    stats: null, // HTML upload status
    form: null, // HTML upload form
    init : function () {
        upload.dropbox = document.getElementById("dropbox");
        upload.stats = document.getElementById("statistics");
        upload.form = document.getElementById("odmUpload");
        
        if (window.File && window.FileReader && window.FileList && window.Blob) {
            upload.dropbox.addEventListener("dragenter", function (e) {
                e.preventDefault();
                e.stopPropagation();
                upload.dropbox.classList.add('hover');
            });
            upload.dropbox.addEventListener("dragleave", function (e) {
                e.preventDefault();
                e.stopPropagation();
                upload.dropbox.classList.remove('hover');
            });
            
            upload.dropbox.addEventListener("dragover", function (e) {
                e.preventDefault();
                e.stopPropagation();
            });
            upload.dropbox.addEventListener("drop", function (e) {
                e.preventDefault();
                e.stopPropagation();
                upload.dropbox.classList.remove('hover');
                var data = e.dataTransfer, files = data.files;
                if (files.length > 1) {
                    errorMessage(upload.dropbox, "Only 1 file may be uploaded")
                    return
                }
                // Validate the file
                if (!files[0].name.endsWith(".odm")) {
                    errorMessage(upload.dropbox, "Only '.odm' files may be uploaded")
                    return
                }
                if (files[0].size > 9999) {
                    errorMessage(upload.dropbox, "File exceeds size limit")
                    return
                }
                document.getElementById("fileInput").files = files
                upload.form.submit()
            });
        }

        else {
            upload.dropbox.style.display = "none";
            upload.form.style.display = "block";
        }
    }
}
window.addEventListener("DOMContentLoaded", upload.init);