# godm
Download MP3 from overdrive odm files for local playback

## Getting started
This will take an audiobook you have checked out from overdrive and convert it to a collection of mp3 files.

1. Install the [docker](https://docs.docker.com/get-docker/) app. This lets you run a docker file on your machine (similar to a virtual machine), and you will need this in order to run the code in this repository.
   - Start the docker app and let it continue running

2. Get the code. Make sure you have [git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git) installed, then using a terminal and git, run:
```
clone https://github.com/micahjmartin/godm
```

3. Prepare the code. From a terminal:

   build the docker image
   ```
   cd godm
   docker build ./
   ```
   show the docker image name you just built
   ```
   docker image ls
   ```
4. Run the code

   ```
   docker run -p 3000:8080 -v `pwd`/audiobooks:/app/output {YOUR IMAGE ID}
   ```

   Now the docker is running locally on your machine. A local web server is listening on port `3000` (mapped to `8080` in the docker) and will output converted audiobooks to your local directory `audiobooks` (`/app/output` in the docker).

5. For every audiobook
   - Download the `.odm` files
      - Log into overdrive.com, go to your loans
      - Download the `.odm` file by expanding the "Do you have the OverDrive app?" button and clicking the download link.
     - If you are not on a Windows computer, you need to view the page as if you were on Windows. If using Safari, go to preferences and enable developer settings (only need to do this once). Then, from the bookshelf page, select Develop-->User Agent->Microsoft Edge (Windows). The page should reload and the button should appear. 
     You now have a .odm file in your downloads. This is not the audiobook, but a special file that points to it. The next steps will convert this file to a collection of mp3 files.
   - Go to http://localhost:3000. You should see a simple webpage where you can drag-and-drop odm files
   - Drag and drop a single odm file to this web page. You can watch activity log to your terminal. Wait for “Success” to be printed from the terminal
   - Copy the zip file from your local `audiobooks` directory to somewhere else because you may lose access to them when the docker is stopped.
6. Unzip the copied files to get the final audiobook mp3 files
7. You can stop the docker by pressing CTRL-C. You can return to step 4 the next time you need to convert more audiobooks

