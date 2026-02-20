class MediaUrlBuilder
  def self.call(stream_name:)
    new(stream_name:).call
  end

  def initialize(stream_name:)
    @stream_name = stream_name.to_s.gsub(%r{\A/+}, "")
  end

  def call
    {
      webrtc: "#{webrtc_base}/#{stream_name}/whep",
      hls: "#{hls_base}/#{stream_name}/index.m3u8",
      rtsp: "#{rtsp_base}/#{stream_name}",
      rtmp: "#{rtmp_base}/#{stream_name}"
    }
  end

  private

  attr_reader :stream_name

  def webrtc_base
    ENV.fetch("MEDIA_WEBRTC_BASE_URL", "http://localhost:8889")
  end

  def hls_base
    ENV.fetch("MEDIA_HLS_BASE_URL", "http://localhost:8888")
  end

  def rtsp_base
    ENV.fetch("MEDIA_RTSP_BASE_URL", "rtsp://localhost:8554")
  end

  def rtmp_base
    ENV.fetch("MEDIA_RTMP_BASE_URL", "rtmp://localhost:1935")
  end
end
